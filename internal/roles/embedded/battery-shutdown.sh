#!/usr/bin/env bash
#
# UPS Battery Remote Shutdown Daemon
# Polls battery charge and SSHes to remote nodes when it drops below a threshold.
# Designed to run as a systemd service on the NUT server (the Pi).
#
# Usage: battery-shutdown.sh [--test]
#   --test   do a dry-run (prints what it would do, no SSH)
#
# Configuration: /etc/ups-battery-shutdown.conf
#   UPS=myups@localhost
#   THRESHOLD=50
#   REMOTE_NODES="user@host1 admin@unifi-device"   (space-separated)
#   POLL_INTERVAL=30
#   REMOTE_SHUTDOWN_CMD=~/shutdown.sh              (default for all nodes)
#   CMD_unifi_device=poweroff                      (per-node override; hyphens→underscores)
#
# Per-node CMD overrides: UniFi devices don't persist scripts across firmware
# updates, so set CMD_<hostname> to an inline command (e.g. poweroff) instead.
#
set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--test" ]] && DRY_RUN=1

CONF="/etc/ups-battery-shutdown.conf"
# systemd's RuntimeDirectory=ups-battery-shutdown creates /run/ups-battery-shutdown
# owned by the service user, so the unprivileged daemon can write its lock here.
LOCK_FILE="/run/ups-battery-shutdown/lock"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes"

# Defaults (overridden by conf file)
UPS="myups@localhost"
THRESHOLD=50
REMOTE_NODES=""
POLL_INTERVAL=30
REMOTE_SHUTDOWN_CMD="~/shutdown.sh"
LOG_FILE=""
SLACK_WEBHOOK=""
SSH_KEY=""

[[ -f "$CONF" ]] && source "$CONF"

# Use the daemon's dedicated key when configured. The daemon runs as the
# unprivileged 'homelab-nut' service user, whose key lives at
# /var/lib/homelab-nut/.ssh/id_ed25519_ups; without pointing -i at it the SSH
# below has no identity to offer and auth fails. SSH_OPTS is intentionally
# word-split in the ssh calls, so SSH_KEY must be path-only.
if [[ -n "$SSH_KEY" ]]; then
    SSH_OPTS="-i $SSH_KEY $SSH_OPTS"
fi

# Date-stamp the log filename (foo.log → foo-2026-05-14.log) so each day gets its own file
if [[ -n "$LOG_FILE" ]]; then
    _log_dir="$(dirname "$LOG_FILE")"
    _log_base="$(basename "$LOG_FILE" .log)"
    LOG_FILE="${_log_dir}/${_log_base}-$(date '+%Y-%m-%d').log"
fi

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    logger -t ups-battery-shutdown "$*"
    echo "$msg"
    [[ -n "$LOG_FILE" ]] && echo "$msg" >> "$LOG_FILE"
}

slack() {
    [[ -z "$SLACK_WEBHOOK" ]] && return
    local text="$*"
    curl -s -X POST "$SLACK_WEBHOOK" \
        -H 'Content-type: application/json' \
        -d "{\"text\": \"${text}\"}" >/dev/null 2>&1 || true
}

if [[ -z "$REMOTE_NODES" ]]; then
    log "ERROR: REMOTE_NODES not configured in $CONF"
    exit 1
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY RUN mode — no SSH commands will be sent"
fi

log "UPS battery watcher started"
log "  UPS:       $UPS"
log "  Threshold: ${THRESHOLD}%"
log "  Nodes:     $REMOTE_NODES"
log "  Interval:  ${POLL_INTERVAL}s"

FAIL_COUNT=0
FAIL_WARN=5   # log a warning after this many consecutive failed polls

while true; do
    STATUS=$(upsc "$UPS" ups.status 2>/dev/null || echo "")
    CHARGE=$(upsc "$UPS" battery.charge 2>/dev/null || echo "")

    if [[ -z "$STATUS" ]] || [[ -z "$CHARGE" ]]; then
        FAIL_COUNT=$(( FAIL_COUNT + 1 ))
        if (( FAIL_COUNT == 1 )) || (( FAIL_COUNT % FAIL_WARN == 0 )); then
            log "WARNING: cannot read UPS data from $UPS (consecutive failures: $FAIL_COUNT)"
        fi
        sleep "$POLL_INTERVAL"
        continue
    fi
    FAIL_COUNT=0

    # Clear lock when power is restored so future outages re-trigger
    if [[ "$STATUS" == *"OL"* ]] && [[ -f "$LOCK_FILE" ]]; then
        log "Power restored (OL) — clearing shutdown lock"
        rm -f "$LOCK_FILE"
        slack ":white_check_mark: *$(hostname)* — Power restored. UPS back on mains."
    fi

    if [[ "$STATUS" == *"OB"* ]] && [[ -n "$CHARGE" ]] && \
       [[ "$CHARGE" -le "$THRESHOLD" ]] && [[ ! -f "$LOCK_FILE" ]]; then
        touch "$LOCK_FILE"
        log "Battery at ${CHARGE}% on battery (threshold: ${THRESHOLD}%) — sending remote shutdown"
        slack ":warning: *$(hostname)* — Battery at *${CHARGE}%* (threshold: ${THRESHOLD}%). Shutting down: ${REMOTE_NODES}"

        for NODE in $REMOTE_NODES; do
            HOST="${NODE##*@}"
            SANITIZED="${HOST//-/_}"; SANITIZED="${SANITIZED//\./_}"
            NODE_CMD_VAR="CMD_${SANITIZED}"
            NODE_CMD="${!NODE_CMD_VAR:-$REMOTE_SHUTDOWN_CMD}"

            log "→ Shutting down $NODE via SSH (cmd: $NODE_CMD)..."

            if [[ "$DRY_RUN" -eq 1 ]]; then
                if [[ "$NODE_CMD" == *"~/"* || "$NODE_CMD" == *".sh"* ]]; then
                    log "  [DRY RUN] would run: ssh $SSH_OPTS $NODE 'nohup bash -c \"$NODE_CMD\" >/tmp/ups-shutdown.log 2>&1 </dev/null &'"
                else
                    log "  [DRY RUN] would run: ssh $SSH_OPTS $NODE '$NODE_CMD'"
                fi
            else
                if [[ "$NODE_CMD" == *"~/"* || "$NODE_CMD" == *".sh"* ]]; then
                    # Script — detach via nohup so SSH exits before the machine powers off
                    # shellcheck disable=SC2086
                    SSH_OUT=$(ssh $SSH_OPTS "$NODE" \
                        "nohup bash -c '$NODE_CMD' >/tmp/ups-shutdown.log 2>&1 </dev/null &" \
                        2>&1) && SSH_RC=0 || SSH_RC=$?
                    [[ -n "$SSH_OUT" ]] && log "  ssh($NODE): $SSH_OUT"
                    if [[ "$SSH_RC" -eq 0 ]]; then
                        log "  ✓ Shutdown dispatched to $NODE (check /tmp/ups-shutdown.log there)"
                    else
                        log "  ✗ Failed to reach $NODE (SSH exit $SSH_RC)"
                        slack ":x: *$(hostname)* — Failed to SSH to \`$NODE\` (exit $SSH_RC)"
                    fi
                else
                    # Inline command (e.g. poweroff for UniFi) — run direct, no nohup needed
                    # shellcheck disable=SC2086
                    SSH_OUT=$(ssh $SSH_OPTS "$NODE" "$NODE_CMD" 2>&1) && SSH_RC=0 || SSH_RC=$?
                    [[ -n "$SSH_OUT" ]] && log "  ssh($NODE): $SSH_OUT"
                    if [[ "$SSH_RC" -eq 0 ]]; then
                        log "  ✓ Shutdown command sent to $NODE"
                    else
                        log "  ✗ Failed to reach $NODE (SSH exit $SSH_RC)"
                        slack ":x: *$(hostname)* — Failed to SSH to \`$NODE\` (exit $SSH_RC)"
                    fi
                fi
            fi
        done
    fi

    sleep "$POLL_INTERVAL"
done
