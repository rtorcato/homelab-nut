#!/usr/bin/env bash
#
# Harden file permissions for sensitive NUT files and scripts.
# Run this on the Pi (NUT server) after setup.
#
# Usage: sudo ./harden.sh [--check]
#   --check   audit current permissions without changing anything
#
set -euo pipefail

[[ $EUID -ne 0 ]] && { echo "Error: run as root: sudo $0" >&2; exit 1; }

CHECK_ONLY=0
[[ "${1:-}" == "--check" ]] && CHECK_ONLY=1

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
ok()   { echo -e "  ${GREEN}✓${NC} $*"; }
warn() { echo -e "  ${YELLOW}!${NC} $*"; }
info() { echo -e "  ${CYAN}»${NC} $*"; }

check_or_set() {
    local label="$1" path="$2" want_mode="$3" want_owner="${4:-}"
    [[ ! -e "$path" ]] && { warn "Not found: $path"; return; }

    local cur_mode cur_owner
    cur_mode=$(stat -c '%a' "$path" 2>/dev/null || stat -f '%OLp' "$path")
    cur_owner=$(stat -c '%U' "$path" 2>/dev/null || stat -f '%Su' "$path")

    local mode_ok=1 owner_ok=1
    [[ "$cur_mode" != "$want_mode" ]] && mode_ok=0
    [[ -n "$want_owner" && "$cur_owner" != "$want_owner" ]] && owner_ok=0

    if [[ "$mode_ok" -eq 1 && "$owner_ok" -eq 1 ]]; then
        ok "$label — ${cur_mode} ${cur_owner} (ok)"
        return
    fi

    if [[ "$CHECK_ONLY" -eq 1 ]]; then
        warn "$label — currently ${cur_mode} ${cur_owner}, want ${want_mode} ${want_owner:-any}"
        return
    fi

    [[ "$mode_ok"  -eq 0 ]] && chmod "$want_mode" "$path"
    [[ "$owner_ok" -eq 0 && -n "$want_owner" ]] && chown "$want_owner" "$path"
    ok "$label — set to ${want_mode} ${want_owner:-}"
}

echo
echo -e "${CYAN}=== NUT Permission Hardening ===${NC}"
[[ "$CHECK_ONLY" -eq 1 ]] && echo -e "  ${YELLOW}Check-only mode — no changes will be made${NC}"
echo

echo -e "${CYAN}─ Credentials ─${NC}"
check_or_set "nut-credentials.txt"    /root/nut-credentials.txt     600 root

echo
echo -e "${CYAN}─ NUT config files ─${NC}"
check_or_set "upsd.conf"              /etc/nut/upsd.conf             640 root
check_or_set "upsd.users"            /etc/nut/upsd.users             640 root
check_or_set "upsmon.conf"           /etc/nut/upsmon.conf            640 root
check_or_set "ups.conf"              /etc/nut/ups.conf               640 root
check_or_set "ups-battery-shutdown.conf" /etc/ups-battery-shutdown.conf 640 root

echo
echo -e "${CYAN}─ SSH key (UPS) ─${NC}"
check_or_set "id_ed25519_ups"        /root/.ssh/id_ed25519_ups       600 root
check_or_set "id_ed25519_ups.pub"    /root/.ssh/id_ed25519_ups.pub   644 root

echo
echo -e "${CYAN}─ Installed daemon ─${NC}"
check_or_set "ups-battery-shutdown"  /usr/local/bin/ups-battery-shutdown 700 root

echo
echo -e "${CYAN}─ Repo scripts ─${NC}"
check_or_set "show-credentials.sh"  "${REPO_ROOT}/scripts/show-credentials.sh"  700
check_or_set "ups-service.sh"       "${REPO_ROOT}/scripts/ups-service.sh"        700
check_or_set "harden.sh"            "${REPO_ROOT}/scripts/harden.sh"             700
check_or_set "setup-server.sh"      "${REPO_ROOT}/scripts/setup-server.sh"       750
check_or_set "setup-client.sh"      "${REPO_ROOT}/scripts/setup-client.sh"       750
check_or_set "setup-exporter.sh"    "${REPO_ROOT}/scripts/setup-exporter.sh"     750

echo
if [[ "$CHECK_ONLY" -eq 0 ]]; then
    echo -e "  ${GREEN}Done.${NC} Re-run with --check to audit at any time."
else
    echo -e "  Run ${CYAN}sudo $0${NC} (without --check) to apply fixes."
fi
echo
