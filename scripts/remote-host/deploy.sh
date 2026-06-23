#!/usr/bin/env bash
#
# Deploy shutdown.sh to remote nodes and configure passwordless sudo for shutdown.
# Run this from the Pi or any machine with SSH access to the target nodes.
#
# Usage: ./deploy.sh user@host [user@host ...]
#
# What it does on each node:
#   1. Copies shutdown.sh → ~/shutdown.sh
#   2. Sets permissions to 700
#   3. Configures passwordless sudo for shutdown (/etc/sudoers.d/ups-shutdown)
#   4. Runs ~/shutdown.sh --test to verify
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SHUTDOWN_SCRIPT="${SCRIPT_DIR}/shutdown.sh"

SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10"
SSH_KEY="/var/lib/homelab-nut/.ssh/id_ed25519_ups"
[[ -f "$SSH_KEY" ]] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

[[ $# -eq 0 ]] && { echo "Usage: $0 user@host [user@host ...]" >&2; exit 1; }
[[ ! -f "$SHUTDOWN_SCRIPT" ]] && { echo "Error: shutdown.sh not found at $SHUTDOWN_SCRIPT" >&2; exit 1; }

for NODE in "$@"; do
    echo
    echo "=== Deploying to $NODE ==="

    # shellcheck disable=SC2086
    scp $SSH_OPTS "$SHUTDOWN_SCRIPT" "${NODE}:~/shutdown.sh"
    echo "  Copied shutdown.sh"

    # shellcheck disable=SC2086
    ssh $SSH_OPTS "$NODE" 'chmod 700 ~/shutdown.sh'
    echo "  Set permissions to 700"

    RUSER="${NODE%%@*}"
    # -t allocates a TTY so sudo can prompt for password interactively
    # shellcheck disable=SC2086
    ssh -t $SSH_OPTS "$NODE" \
        "echo '${RUSER} ALL=(ALL) NOPASSWD: /sbin/shutdown' | sudo tee /etc/sudoers.d/ups-shutdown >/dev/null"
    echo "  Configured passwordless sudo for shutdown"

    echo "  Running test..."
    # shellcheck disable=SC2086
    ssh $SSH_OPTS "$NODE" '~/shutdown.sh --test'

    echo "  Done: $NODE"
done

echo
echo "All nodes deployed. The Remote will run ~/shutdown.sh on each node during a UPS event."
