#!/usr/bin/env bash
# Show NUT credentials stored on this host.
set -euo pipefail

CREDS_FILE="/root/nut-credentials.txt"

if [ ! -f "$CREDS_FILE" ]; then
    echo "Error: $CREDS_FILE not found." >&2
    echo "Run scripts/setup-server.sh to generate credentials." >&2
    exit 1
fi

sudo cat "$CREDS_FILE"
