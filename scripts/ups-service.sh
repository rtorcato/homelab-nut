#!/usr/bin/env bash
#
# UPS Remote Shutdown
# Run on the Pi (NUT server) to set up or manage the ups-battery-shutdown service.
#
# First run:  interactive setup wizard (SSH keys, remote nodes, threshold)
# Later runs: management menu (start/stop/restart/threshold/edit/logs/remove)
#
# Usage: sudo ./ups-service.sh [command]
#
# Commands (optional — omit for interactive mode):
#   status              Show service status and config
#   start               Start the service
#   stop                Stop the service
#   restart             Restart the service
#   set-threshold <N>   Set battery charge threshold to N%
#   edit                Open config in $EDITOR
#   logs                Tail service logs
#   setup               Re-run the setup wizard
#   remove              Uninstall the service
#
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
err()  { echo -e "\n${RED}Error:${NC} $*" >&2; exit 1; }
ok()   { echo -e "  ${GREEN}✓${NC} $*"; }
info() { echo -e "  ${CYAN}»${NC} $*"; }
warn() { echo -e "  ${YELLOW}!${NC} $*"; }

[[ $EUID -ne 0 ]] && err "Run as root: sudo $0 $*"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONF_REPO="${REPO_ROOT}/config/ups-battery-shutdown.conf"
CONF="/etc/ups-battery-shutdown.conf"   # symlink → CONF_REPO
DAEMON_SRC="$(dirname "$0")/battery-shutdown.sh"
DAEMON_DST="/usr/local/bin/ups-battery-shutdown"
SERVICE_FILE="/etc/systemd/system/ups-battery-shutdown.service"
SERVICE="ups-battery-shutdown.service"
SSH_KEY="/root/.ssh/id_ed25519_ups"

# ── Helpers ───────────────────────────────────────────────────────────────────

service_installed() { [[ -f "$SERVICE_FILE" ]]; }
service_running()   { systemctl is-active --quiet "$SERVICE" 2>/dev/null; }

show_status() {
    echo
    echo -e "${CYAN}${BOLD}=== UPS Remote Shutdown ===${NC}"
    echo
    if service_installed; then
        if service_running; then
            echo -e "  Service:   ${GREEN}running${NC}"
        else
            echo -e "  Service:   ${RED}stopped${NC}"
        fi
        if systemctl is-enabled --quiet "$SERVICE" 2>/dev/null; then
            echo -e "  Autostart: ${GREEN}enabled${NC}"
        else
            echo -e "  Autostart: ${YELLOW}disabled${NC}"
        fi
    else
        echo -e "  Service:   ${YELLOW}not installed${NC}"
    fi

    if [[ -f "$CONF_REPO" ]]; then
        echo
        while IFS='=' read -r key val; do
            [[ "$key" =~ ^#|^[[:space:]]*$ ]] && continue
            printf "  %-20s %s\n" "${key}:" "$val"
        done < "$CONF_REPO"

        # Live battery reading
        source "$CONF_REPO" 2>/dev/null || true
        LIVE_CHARGE=$(upsc "${UPS:-myups@localhost}" battery.charge 2>/dev/null || echo "")
        LIVE_STATUS=$(upsc "${UPS:-myups@localhost}" ups.status  2>/dev/null || echo "")
        [[ -n "$LIVE_CHARGE" ]] && echo -e "\n  Battery:   ${LIVE_CHARGE}% (${LIVE_STATUS})"
    fi
    echo
}

do_set_threshold() {
    local N="${1:-}"
    [[ -z "$N" ]] && err "Usage: sudo $0 set-threshold <number>"
    [[ ! "$N" =~ ^[0-9]+$ ]] || [[ "$N" -lt 1 ]] || [[ "$N" -gt 99 ]] && \
        err "Threshold must be a number between 1 and 99"
    [[ ! -f "$CONF_REPO" ]] && err "Config not found: $CONF_REPO — run setup first"
    sed -i "s/^THRESHOLD=.*/THRESHOLD=${N}/" "$CONF_REPO"
    ok "Threshold set to ${N}%"
    systemctl restart "$SERVICE"
    ok "Service restarted"
}

do_edit() {
    [[ ! -f "$CONF_REPO" ]] && err "Config not found: $CONF_REPO — run setup first"
    ${EDITOR:-nano} "$CONF_REPO"
    echo
    read -r -p "  Restart service to apply changes? [Y/n] " REPLY
    REPLY="${REPLY:-Y}"
    [[ "$REPLY" =~ ^[Yy]$ ]] && systemctl restart "$SERVICE" && ok "Service restarted"
}

do_remove() {
    echo
    warn "This will stop, disable, and remove the ups-battery-shutdown service."
    warn "Removed: $DAEMON_DST  $CONF  $SERVICE_FILE"
    warn "Kept:    $CONF_REPO (repo config — delete manually if needed)"
    echo
    read -r -p "  Are you sure? [y/N] " REPLY
    [[ ! "$REPLY" =~ ^[Yy]$ ]] && { info "Aborted."; return 0; }
    systemctl stop    "$SERVICE" 2>/dev/null && ok "Service stopped"   || true
    systemctl disable "$SERVICE" 2>/dev/null && ok "Service disabled"  || true
    rm -f "$SERVICE_FILE" && ok "Removed unit file"
    rm -f "$DAEMON_DST"   && ok "Removed daemon"
    rm -f "$CONF"         && ok "Removed /etc symlink"
    rm -f "/run/ups-battery-shutdown.lock"
    systemctl daemon-reload
    echo
    ok "Service fully removed"
    echo
}

# ── Setup wizard ──────────────────────────────────────────────────────────────

run_setup() {
    echo
    echo -e "${CYAN}${BOLD}=== UPS Remote Shutdown Setup ===${NC}"
    echo

    # Remote nodes
    read -r -p "  Remote node(s) to shut down [user@host, space-separated]: " REMOTE_NODES
    [[ -z "$REMOTE_NODES" ]] && err "At least one remote node is required"

    # Threshold
    read -r -p "  Shutdown threshold % [default 50]: " THRESHOLD
    THRESHOLD="${THRESHOLD:-50}"
    [[ ! "$THRESHOLD" =~ ^[0-9]+$ ]] || [[ "$THRESHOLD" -lt 1 ]] || [[ "$THRESHOLD" -gt 99 ]] && \
        err "Threshold must be 1–99"

    # Auto-detect UPS
    UPS=$(upsc -l localhost 2>/dev/null | head -n1 || true)
    [[ -z "$UPS" ]] && UPS="myups"
    UPS="${UPS}@localhost"

    echo
    echo "  Nodes:     $REMOTE_NODES"
    echo "  Threshold: ${THRESHOLD}%"
    echo "  UPS:       $UPS"
    echo

    # ── Resolve hostnames ──────────────────────────────────────────────────────
    for NODE in $REMOTE_NODES; do
        HOST="${NODE##*@}"
        if ! getent hosts "$HOST" &>/dev/null && ! ping -c1 -W2 "$HOST" &>/dev/null; then
            warn "Cannot resolve '$HOST' as root (DNS/mDNS may not be available to root)."
            read -r -p "  Enter IP address for $HOST: " NODE_IP
            if [[ -n "$NODE_IP" ]]; then
                if grep -qE "\\s${HOST}$" /etc/hosts; then
                    warn "$HOST already in /etc/hosts — skipping"
                else
                    echo "$NODE_IP  $HOST" >> /etc/hosts
                    ok "Added '$NODE_IP $HOST' to /etc/hosts"
                fi
            else
                err "IP required — re-run setup"
            fi
        fi
    done

    # ── SSH key ────────────────────────────────────────────────────────────────
    info "Checking SSH key..."
    mkdir -p /root/.ssh && chmod 700 /root/.ssh
    if [[ ! -f "$SSH_KEY" ]]; then
        ssh-keygen -t ed25519 -f "$SSH_KEY" -N "" -C "ups-battery-shutdown@$(hostname)"
        ok "Generated $SSH_KEY"
    else
        ok "SSH key already exists: $SSH_KEY"
    fi

    # ── Copy key to each node ──────────────────────────────────────────────────
    for NODE in $REMOTE_NODES; do
        info "Copying SSH key to $NODE (you will be prompted for the password)..."
        if ssh-copy-id -i "${SSH_KEY}.pub" "$NODE"; then
            ok "Key copied to $NODE"
        else
            warn "ssh-copy-id failed. Add this key manually to ~/.ssh/authorized_keys on $NODE:"
            echo
            echo "    $(cat "${SSH_KEY}.pub")"
            echo
            read -r -p "  Press Enter once the key is in place..."
        fi
    done

    # ── Test SSH ───────────────────────────────────────────────────────────────
    echo
    for NODE in $REMOTE_NODES; do
        info "Testing SSH to $NODE..."
        SSH_ERR=$(ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
                      -o BatchMode=yes "$NODE" 'echo ok' 2>&1) && SSH_OK=1 || SSH_OK=0
        if [[ "$SSH_OK" -eq 1 ]]; then
            ok "SSH to $NODE works"
        else
            echo -e "  ${RED}SSH test failed:${NC} $SSH_ERR"
            echo
            warn "Troubleshooting:"
            echo "    1. Reachable?   ping ${NODE##*@}"
            echo "    2. SSH running? ssh $NODE"
            echo "    3. Key in place? check ~/.ssh/authorized_keys on $NODE"
            echo "    4. Non-standard port? add -p <port> to SSH_OPTS in $DAEMON_DST after setup"
            err "Fix SSH access to $NODE and re-run: sudo $0 setup"
        fi
    done

    # ── Passwordless sudo ──────────────────────────────────────────────────────
    echo
    for NODE in $REMOTE_NODES; do
        RUSER="${NODE%%@*}"
        info "Checking passwordless sudo on $NODE..."
        if ssh -i "$SSH_KEY" -o BatchMode=yes -o ConnectTimeout=10 \
               "$NODE" 'sudo -n shutdown --help' &>/dev/null; then
            ok "sudo shutdown works on $NODE"
        else
            warn "sudo shutdown not yet configured on $NODE. Run this on $NODE:"
            echo
            echo "    echo '${RUSER} ALL=(ALL) NOPASSWD: /sbin/shutdown' | sudo tee /etc/sudoers.d/ups-shutdown"
            echo
            read -r -p "  Press Enter once configured (or to skip)..."
        fi
    done

    # ── Install daemon ─────────────────────────────────────────────────────────
    echo
    [[ ! -f "$DAEMON_SRC" ]] && err "Cannot find battery-shutdown.sh at $DAEMON_SRC"
    install -m 755 "$DAEMON_SRC" "$DAEMON_DST"
    ok "Installed $DAEMON_DST"

    # ── Write config ───────────────────────────────────────────────────────────
    local SKIP_CONF=0
    if [[ -f "$CONF_REPO" ]]; then
        warn "Config already exists: $CONF_REPO"
        echo
        grep -v '^#' "$CONF_REPO" | grep -v '^[[:space:]]*$' | sed 's/^/    /'
        echo
        read -r -p "  Overwrite with new values? [y/N] " REPLY
        [[ ! "$REPLY" =~ ^[Yy]$ ]] && SKIP_CONF=1
    fi

    if [[ "$SKIP_CONF" -eq 0 ]]; then
        mkdir -p "${REPO_ROOT}/config"
        cat > "$CONF_REPO" << EOF
# UPS Battery Remote Shutdown Configuration
# Generated by ups-service.sh on $(date)

UPS=${UPS}
THRESHOLD=${THRESHOLD}
REMOTE_NODES="${REMOTE_NODES}"
POLL_INTERVAL=30
SSH_KEY=${SSH_KEY}
REMOTE_SHUTDOWN_CMD=~/shutdown.sh
EOF
        chmod 640 "$CONF_REPO"
        ok "Config written: $CONF_REPO"
    else
        ok "Keeping existing config"
    fi

    # ── Symlink config into /etc ───────────────────────────────────────────────
    ln -sf "$CONF_REPO" "$CONF"
    ok "Symlinked $CONF → $CONF_REPO"

    # ── Patch SSH key into daemon ──────────────────────────────────────────────
    if ! grep -q "id_ed25519_ups" "$DAEMON_DST"; then
        sed -i "s|SSH_OPTS=\"|SSH_OPTS=\"-i ${SSH_KEY} |" "$DAEMON_DST"
    fi

    # ── Systemd service ────────────────────────────────────────────────────────
    info "Creating systemd service..."
    cat > "$SERVICE_FILE" << 'UNIT'
[Unit]
Description=UPS Battery Remote Shutdown Watcher
After=nut-server.service nut-monitor.service network.target
Wants=nut-server.service

[Service]
Type=simple
ExecStart=/usr/local/bin/ups-battery-shutdown
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT

    systemctl daemon-reload
    systemctl enable "$SERVICE"
    systemctl restart "$SERVICE"
    ok "Service enabled and started"

    echo
    show_status
    warn "When battery hits ${THRESHOLD}% on battery, the Pi will SSH to [$REMOTE_NODES] and run '~/shutdown.sh'."
    echo
    echo "  Manage anytime:  sudo $0"
    echo "  Dry-run test:    $DAEMON_DST --test"
    echo
}

# ── Interactive management menu ───────────────────────────────────────────────

run_menu() {
    while true; do
        show_status

        if service_running; then
            echo "  1) Stop service"
        else
            echo "  1) Start service"
        fi
        echo "  2) Restart service"
        echo "  3) Set threshold"
        echo "  4) Edit config"
        echo "  5) View logs"
        echo "  6) Re-run setup wizard"
        echo "  7) Remove service"
        echo "  q) Quit"
        echo
        read -r -p "  Choice: " CHOICE
        echo

        case "$CHOICE" in
            1)
                if service_running; then
                    systemctl stop  "$SERVICE" && ok "Service stopped"
                else
                    systemctl start "$SERVICE" && ok "Service started"
                fi
                ;;
            2) systemctl restart "$SERVICE" && ok "Service restarted" ;;
            3)
                read -r -p "  New threshold % (1–99): " N
                do_set_threshold "$N"
                ;;
            4) do_edit ;;
            5) journalctl -u "$SERVICE" -f || true ;;
            6) run_setup; return ;;
            7) do_remove; return ;;
            q|Q) break ;;
            *) warn "Unknown option: $CHOICE" ;;
        esac
    done
}

# ── Entry point ───────────────────────────────────────────────────────────────

CMD="${1:-}"

case "$CMD" in
    status)        show_status ;;
    start)         systemctl start   "$SERVICE" && ok "Started" ;;
    stop)          systemctl stop    "$SERVICE" && ok "Stopped" ;;
    restart)       systemctl restart "$SERVICE" && ok "Restarted" ;;
    logs)          journalctl -u "$SERVICE" -f ;;
    set-threshold) do_set_threshold "${2:-}" ;;
    edit)          do_edit ;;
    remove)        do_remove ;;
    setup)         run_setup ;;
    -h|--help|help)
        sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
        ;;
    "")
        if service_installed; then
            run_menu
        else
            echo
            echo -e "  ${YELLOW}No ups-battery-shutdown service found.${NC}"
            echo
            read -r -p "  Set up UPS remote shutdown now? [Y/n] " REPLY
            REPLY="${REPLY:-Y}"
            if [[ "$REPLY" =~ ^[Yy]$ ]]; then
                run_setup
            else
                info "Nothing to do."
            fi
        fi
        ;;
    *)
        err "Unknown command: $CMD  (try: sudo $0 help)"
        ;;
esac
