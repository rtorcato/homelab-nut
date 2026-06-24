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
#   THRESHOLD=50                                   (default; per-node THRESHOLD_ overrides it)
#   REMOTE_NODES="user@host1 admin@unifi-device"   (space-separated)
#   POLL_INTERVAL=30
#   REMOTE_SHUTDOWN_CMD=~/shutdown.sh              (default for all nodes)
#   CMD_unifi_device=poweroff                      (per-node override; hyphens→underscores)
#   DELAY_unifi_device=60                          (per-node delay in seconds before sending)
#   THRESHOLD_unifi_device=20                      (per-node battery %; staged shutdown)
#
# Per-node CMD overrides: UniFi devices don't persist scripts across firmware
# updates, so set CMD_<hostname> to an inline command (e.g. poweroff) instead.
#
# Per-node DELAY overrides: the daemon waits DELAY_<hostname> seconds before
# sending that node's command, so dependent devices can be sequenced — e.g.
# give a NAS time to finish before powering off the gateway it talks through.
#
# Per-node THRESHOLD overrides: each node fires as the UPS crosses its own
# THRESHOLD_<hostname> percent (staged shutdown — shed a NAS early at 60%,
# the router last at 20%). Nodes without one use the global THRESHOLD. Each
# node is fired at most once per outage (per-node lock files), cleared on OL.
#
set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--test" ]] && DRY_RUN=1

CONF="/etc/ups-battery-shutdown.conf"
# systemd's RuntimeDirectory=ups-battery-shutdown creates /run/ups-battery-shutdown
# owned by the service user, so the unprivileged daemon can write its locks here.
# One lock per node (lock-<sanitized-host>) so staged targets fire independently.
LOCK_DIR="/run/ups-battery-shutdown"
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
log "  Threshold: ${THRESHOLD}% (default; per-node THRESHOLD_ overrides apply)"
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

    # Clear per-node locks when power is restored so future outages re-trigger.
    if [[ "$STATUS" == *"OL"* ]]; then
        if compgen -G "${LOCK_DIR}/lock-*" >/dev/null 2>&1; then
            log "Power restored (OL) — clearing shutdown locks"
            rm -f "${LOCK_DIR}"/lock-*
            slack ":white_check_mark: *$(hostname)* — Power restored. UPS back on mains."
        fi
    fi

    # Staged shutdown: on battery, walk every node and fire any whose own
    # threshold the charge has now crossed and that hasn't fired this outage.
    # Per-node lock files keep each node a one-shot until power is restored.
    if [[ "$STATUS" == *"OB"* ]] && [[ -n "$CHARGE" ]]; then
        for NODE in $REMOTE_NODES; do
            HOST="${NODE##*@}"
            SANITIZED="${HOST//-/_}"; SANITIZED="${SANITIZED//\./_}"
            NODE_THRESHOLD_VAR="THRESHOLD_${SANITIZED}"
            NODE_THRESHOLD="${!NODE_THRESHOLD_VAR:-$THRESHOLD}"
            NODE_LOCK="${LOCK_DIR}/lock-${SANITIZED}"

            # Battery still above this node's threshold — leave it running.
            if [[ "$CHARGE" -gt "$NODE_THRESHOLD" ]]; then
                continue
            fi
            # Already fired this outage.
            if [[ -f "$NODE_LOCK" ]]; then
                continue
            fi
            touch "$NODE_LOCK"

            NODE_CMD_VAR="CMD_${SANITIZED}"
            NODE_CMD="${!NODE_CMD_VAR:-$REMOTE_SHUTDOWN_CMD}"
            NODE_DELAY_VAR="DELAY_${SANITIZED}"
            NODE_DELAY="${!NODE_DELAY_VAR:-0}"

            log "Battery at ${CHARGE}% ≤ ${NODE_THRESHOLD}% — shutting down $NODE"
            slack ":warning: *$(hostname)* — Battery at *${CHARGE}%* (≤ ${NODE_THRESHOLD}%). Shutting down: \`${NODE}\`"

            if [[ "$NODE_DELAY" -gt 0 ]]; then
                log "→ Waiting ${NODE_DELAY}s before shutting down $NODE..."
                [[ "$DRY_RUN" -ne 1 ]] && sleep "$NODE_DELAY"
            fi

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
