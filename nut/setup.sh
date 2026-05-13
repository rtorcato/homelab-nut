#!/usr/bin/env bash
# Install and configure NUT (Network UPS Tools) server on Ubuntu/Debian
# Usage:  chmod +x ./nut/setup.sh
# Usage:  ./nut/setup.sh [--dry-run]
#
# After running: edit /etc/nut/ups.conf to set the correct driver for your UPS.
# Run `sudo nut-scanner -U` to detect USB UPS devices and find the right driver.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "$SCRIPT_DIR/../lib/common.sh"

parse_dry_run "$@"

[[ "$(uname)" == "Linux" ]] || { log_error "This script is for Linux only."; exit 1; }

SUMMARY=()

####################
# Install NUT
log_section "Installing NUT"
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would install nut nut-client"
else
    sudo apt-get update -y
    sudo apt-get install -y nut nut-client
    log_info "NUT installed"
fi
SUMMARY+=("NUT:        installed via apt")

####################
# nut.conf — standalone mode (one machine, USB-connected UPS)
log_section "Writing /etc/nut/nut.conf"
NUT_CONF="/etc/nut/nut.conf"
NUT_CONF_CONTENT='# NUT operating mode
# standalone  — this machine monitors a directly-connected UPS
# netserver   — also serve other machines on the network
# netclient   — monitor a remote NUT server (no local UPS)
MODE=standalone
'
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would write $NUT_CONF"
else
    echo "$NUT_CONF_CONTENT" | sudo tee "$NUT_CONF" > /dev/null
    log_info "Written: $NUT_CONF"
fi
SUMMARY+=("nut.conf:   MODE=standalone")

####################
# ups.conf — UPS device definition (placeholder — user must edit driver)
log_section "Writing /etc/nut/ups.conf"
UPS_CONF="/etc/nut/ups.conf"
UPS_CONF_CONTENT='# UPS device definition
# Run `sudo nut-scanner -U` to detect your USB UPS and find the right driver.
#
# Common drivers:
#   usbhid-ups   — most modern USB UPS (APC, Eaton, CyberPower, etc.)
#   blazer_usb   — many budget USB UPS brands
#   apcsmart     — APC via serial port
#
[myups]
    driver = usbhid-ups
    port = auto
    desc = "My UPS"
'
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would write $UPS_CONF"
else
    if [[ -f "$UPS_CONF" ]] && grep -qv '^\s*#' "$UPS_CONF" 2>/dev/null; then
        sudo cp "$UPS_CONF" "${UPS_CONF}.bak"
        log_warn "Backed up existing ups.conf to ${UPS_CONF}.bak"
    fi
    echo "$UPS_CONF_CONTENT" | sudo tee "$UPS_CONF" > /dev/null
    log_info "Written: $UPS_CONF (edit driver if needed)"
fi
SUMMARY+=("ups.conf:   placeholder written — edit driver if needed")

####################
# upsd.conf — daemon listen address
log_section "Writing /etc/nut/upsd.conf"
UPSD_CONF="/etc/nut/upsd.conf"
UPSD_CONF_CONTENT='# NUT server listen address
# Change to 0.0.0.0 to serve other machines on the network
LISTEN 127.0.0.1 3493
'
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would write $UPSD_CONF"
else
    echo "$UPSD_CONF_CONTENT" | sudo tee "$UPSD_CONF" > /dev/null
    log_info "Written: $UPSD_CONF"
fi
SUMMARY+=("upsd.conf:  listening on 127.0.0.1:3493")

####################
# upsd.users — NUT user accounts
log_section "Writing /etc/nut/upsd.users"
UPSD_USERS="/etc/nut/upsd.users"
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would prompt for NUT passwords and write $UPSD_USERS"
    NUT_ADMIN_PASS="adminpass"
    NUT_MONITOR_PASS="monpass"
else
    echo ""
    read -rs -p "NUT admin password (for upsd.users [admin]): " NUT_ADMIN_PASS; echo
    read -rs -p "NUT monitor password (for upsd.users [monitor]): " NUT_MONITOR_PASS; echo
    echo ""
fi
UPSD_USERS_CONTENT="# NUT user accounts

[admin]
    password = ${NUT_ADMIN_PASS}
    actions = SET
    instcmds = ALL

[monitor]
    password = ${NUT_MONITOR_PASS}
    upsmon master
"
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would write $UPSD_USERS"
else
    echo "$UPSD_USERS_CONTENT" | sudo tee "$UPSD_USERS" > /dev/null
    sudo chmod 640 "$UPSD_USERS"
    log_info "Written: $UPSD_USERS"
fi
SUMMARY+=("upsd.users: admin + monitor accounts written")

####################
# upsmon.conf — monitoring and shutdown policy
log_section "Writing /etc/nut/upsmon.conf"
UPSMON_CONF="/etc/nut/upsmon.conf"
UPSMON_CONF_CONTENT=$(cat <<EOF
# NUT monitor configuration
# MONITOR <ups>@<host> <powervalue> <user> <password> <type>
MONITOR myups@localhost 1 monitor ${NUT_MONITOR_PASS} master

MINSUPPLIES 1
SHUTDOWNCMD "/sbin/shutdown -h now"
NOTIFYCMD /usr/sbin/upssched
POLLFREQ 5
POLLFREQALERT 5
HOSTSYNC 15
DEADTIME 15
POWERDOWNFLAG /etc/killpower

NOTIFYMSG ONLINE     "UPS %s on line power"
NOTIFYMSG ONBATT     "UPS %s on battery"
NOTIFYMSG LOWBATT    "UPS %s battery is low"
NOTIFYMSG FSD        "UPS %s: forced shutdown in progress"
NOTIFYMSG COMMOK     "Communications with UPS %s established"
NOTIFYMSG COMMBAD    "Communications with UPS %s lost"
NOTIFYMSG SHUTDOWN   "Auto logout and shutdown proceeding"
NOTIFYMSG REPLBATT   "UPS %s battery needs to be replaced"
NOTIFYMSG NOCOMM     "UPS %s is unavailable"
NOTIFYMSG NOPARENT   "upsmon parent process died - shutdown impossible"

NOTIFYFLAG ONLINE   SYSLOG+WALL
NOTIFYFLAG ONBATT   SYSLOG+WALL+EXEC
NOTIFYFLAG LOWBATT  SYSLOG+WALL
NOTIFYFLAG FSD      SYSLOG+WALL+EXEC
NOTIFYFLAG COMMOK   SYSLOG+WALL
NOTIFYFLAG COMMBAD  SYSLOG+WALL
NOTIFYFLAG SHUTDOWN SYSLOG+WALL+EXEC
NOTIFYFLAG REPLBATT SYSLOG+WALL
NOTIFYFLAG NOCOMM   SYSLOG+WALL+EXEC
NOTIFYFLAG NOPARENT SYSLOG+WALL

RBWARNTIME 43200
NOCOMMWARNTIME 300
FINALDELAY 5
EOF
)
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would write $UPSMON_CONF"
else
    echo "$UPSMON_CONF_CONTENT" | sudo tee "$UPSMON_CONF" > /dev/null
    sudo chmod 640 "$UPSMON_CONF"
    log_info "Written: $UPSMON_CONF"
fi
SUMMARY+=("upsmon.conf: shutdown + notify policy written")

####################
# Enable and start services
log_section "Enabling NUT services"
if [[ "$DRY_RUN" == true ]]; then
    log_info "[DRY-RUN] Would enable and start nut-server nut-monitor"
else
    sudo systemctl enable nut-server nut-monitor
    sudo systemctl restart nut-server nut-monitor || log_warn "Services failed to start — check ups.conf driver settings"
    log_info "NUT services enabled"
fi
SUMMARY+=("Services:   nut-server + nut-monitor enabled")

####################
log_section "Summary"
for line in "${SUMMARY[@]}"; do
    log_info "$line"
done
echo ""
if [[ "$DRY_RUN" == false ]]; then
    log_info "Next steps:"
    log_info "  1. Plug in your UPS via USB"
    log_info "  2. Run: sudo nut-scanner -U        # detect UPS + find driver"
    log_info "  3. Edit /etc/nut/ups.conf           # set correct driver"
    log_info "  4. Edit /etc/nut/upsd.users         # change passwords"
    log_info "  5. Run: sudo systemctl restart nut-server nut-monitor"
    log_info "  6. Verify: sudo upsc myups"
fi
