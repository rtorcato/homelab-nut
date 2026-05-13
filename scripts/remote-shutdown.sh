#!/usr/bin/env bash
#
# UPS Remote Shutdown Script — runs on the TARGET machine (not the Pi)
# The NUT Pi SSHes in and executes this when the battery drops below threshold.
#
# Install: scp scripts/remote-shutdown.sh user@host:~/shutdown.sh
#          ssh user@host 'chmod +x ~/shutdown.sh && ~/shutdown.sh --test'
#
# Usage: ~/shutdown.sh [--test]
#   --test   dry-run: log what would happen, do not shut down
#
set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--test" ]] && DRY_RUN=1

LOG_FILE="/tmp/ups-shutdown.log"
SHUTDOWN_DELAY=0   # seconds before shutdown; increase if services need more time

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    echo "$msg"
    echo "$msg" >> "$LOG_FILE"
}

log "============================================"
log "UPS shutdown triggered on $(hostname)"
[[ "$DRY_RUN" -eq 1 ]] && log "DRY RUN — no actual shutdown will occur"

# ── Stop Docker containers (graceful) ────────────────────────────────────────
if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
    CONTAINERS=$(docker ps -q 2>/dev/null || true)
    if [[ -n "$CONTAINERS" ]]; then
        log "Stopping Docker containers..."
        if [[ "$DRY_RUN" -eq 0 ]]; then
            docker stop --time 20 $CONTAINERS 2>&1 | while IFS= read -r line; do log "  docker: $line"; done || true
        else
            log "  [DRY RUN] would stop: $CONTAINERS"
        fi
    else
        log "No running Docker containers"
    fi
else
    log "Docker not running or not installed — skipping"
fi

# ── Sync filesystems ──────────────────────────────────────────────────────────
log "Syncing filesystems..."
[[ "$DRY_RUN" -eq 0 ]] && sync

# ── Shut down ─────────────────────────────────────────────────────────────────
if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY RUN complete — would now run: sudo shutdown -h +${SHUTDOWN_DELAY}"
    log "============================================"
    exit 0
fi

if [[ "$SHUTDOWN_DELAY" -gt 0 ]]; then
    log "Shutting down in ${SHUTDOWN_DELAY}s..."
    sleep "$SHUTDOWN_DELAY"
fi

log "Initiating shutdown now"
log "============================================"

sudo shutdown -h now
