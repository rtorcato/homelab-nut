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
#   REMOTE_NODES="myuser@control-node"   (space-separated for multiple nodes)
#   POLL_INTERVAL=30
#
set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--test" ]] && DRY_RUN=1

CONF="/etc/ups-battery-shutdown.conf"
LOCK_FILE="/run/ups-battery-shutdown.lock"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes"

# Defaults (overridden by conf file)
UPS="myups@localhost"
THRESHOLD=50
REMOTE_NODES=""
POLL_INTERVAL=30

[[ -f "$CONF" ]] && source "$CONF"

log() { logger -t ups-battery-shutdown "$*"; echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }

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
    fi

    if [[ "$STATUS" == *"OB"* ]] && [[ -n "$CHARGE" ]] && \
       [[ "$CHARGE" -le "$THRESHOLD" ]] && [[ ! -f "$LOCK_FILE" ]]; then
        touch "$LOCK_FILE"
        log "Battery at ${CHARGE}% on battery (threshold: ${THRESHOLD}%) — sending remote shutdown"

        for NODE in $REMOTE_NODES; do
            log "→ Shutting down $NODE via SSH..."
            if [[ "$DRY_RUN" -eq 1 ]]; then
                log "  [DRY RUN] would run: ssh $SSH_OPTS $NODE 'sudo shutdown -h now'"
            else
                # shellcheck disable=SC2086
                if ssh $SSH_OPTS "$NODE" 'sudo shutdown -h now' 2>&1 | \
                   while IFS= read -r line; do log "  ssh($NODE): $line"; done; then
                    log "  ✓ Shutdown command accepted by $NODE"
                else
                    log "  ✗ Failed to reach $NODE (exit $?)"
                fi
            fi
        done
    fi

    sleep "$POLL_INTERVAL"
done
