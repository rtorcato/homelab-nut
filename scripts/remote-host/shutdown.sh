#!/usr/bin/env bash
#
# UPS Remote Shutdown Script — runs on the TARGET machine when the Pi SSHes in.
#
# Deploy:  scripts/remote-host/deploy.sh user@host
# Manual:  scp scripts/remote-host/shutdown.sh user@host:~/shutdown.sh
#
# Usage: ~/shutdown.sh [--test]
#   --test   dry-run: log what would happen, no actual shutdown
#
# Requires passwordless sudo for shutdown (handled by deploy.sh):
#   echo "$USER ALL=(ALL) NOPASSWD: /sbin/shutdown" | sudo tee /etc/sudoers.d/ups-shutdown
#
set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--test" ]] && DRY_RUN=1

LOG="/tmp/ups-shutdown.log"
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG"; }

log "UPS shutdown triggered on $(hostname)"
[[ "$DRY_RUN" -eq 1 ]] && log "DRY RUN — no shutdown will occur" && exit 0

# Stop Docker containers gracefully before shutdown
if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
    CONTAINERS=$(docker ps -q 2>/dev/null || true)
    if [[ -n "$CONTAINERS" ]]; then
        log "Stopping Docker containers..."
        docker stop --time 20 $CONTAINERS 2>&1 | while IFS= read -r line; do log "  docker: $line"; done || true
    fi
fi

sync
log "Initiating shutdown"
sudo shutdown -h now

# Force off after 60s if ACPI shutdown hasn't completed
(sleep 60 && sudo systemctl poweroff --force --force) &
