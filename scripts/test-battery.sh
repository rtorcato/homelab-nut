#!/usr/bin/env bash
#
# UPS Battery Test
# Trigger a battery self-test on the LOCAL NUT-monitored UPS.
# Refuses remote targets — must be run on the host the UPS is attached to.
#
# Usage: ./test-battery.sh [UPS] [--quick|--deep|--stop|--status|--list] [-y]
#
# Flags:
#   --quick     start quick battery test (default)
#   --deep      start deep battery test
#   --stop      stop a running battery test
#   --status    print last test result and exit
#   --list      list INSTCMDs supported by this UPS and exit
#   -y, --yes   skip the confirmation prompt
#
# Credentials (admin user required for INSTCMDs):
#   NUT_USER / NUT_PASS env vars, or
#   parsed from /root/nut-credentials.txt if running as root, or
#   prompted interactively.
#
set -euo pipefail

UPS=""
ACTION="quick"
ASSUME_YES=0

for arg in "$@"; do
    case "$arg" in
        --quick)   ACTION="quick" ;;
        --deep)    ACTION="deep" ;;
        --stop)    ACTION="stop" ;;
        --status)  ACTION="status" ;;
        --list)    ACTION="list" ;;
        -y|--yes)  ASSUME_YES=1 ;;
        -h|--help)
            sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        -*)
            echo "Unknown option: $arg" >&2
            exit 2
            ;;
        *)
            UPS="$arg"
            ;;
    esac
done

if [ -t 1 ]; then
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
    CYAN='\033[0;36m'; DIM='\033[2m'; NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' CYAN='' DIM='' NC=''
fi

err()  { echo -e "${RED}Error:${NC} $*" >&2; }
info() { echo -e "${CYAN}»${NC} $*"; }
ok()   { echo -e "${GREEN}✓${NC} $*"; }
warn() { echo -e "${YELLOW}!${NC} $*"; }

print_hints() {
    local s
    s=$(basename "$0")
    echo
    echo -e "${CYAN}─ Options ─${NC}"
    echo -e "  ${DIM}$s [UPS]            target a local UPS (default: auto-discover)${NC}"
    echo -e "  ${DIM}$s --quick          start quick battery test (default)${NC}"
    echo -e "  ${DIM}$s --deep           start deep battery test${NC}"
    echo -e "  ${DIM}$s --stop           stop a running battery test${NC}"
    echo -e "  ${DIM}$s --status         show last test result${NC}"
    echo -e "  ${DIM}$s --list           list INSTCMDs the UPS supports${NC}"
    echo -e "  ${DIM}$s -y | --yes       skip confirmation prompt${NC}"
    echo -e "  ${DIM}$s -h | --help      show usage${NC}"
    echo -e "  ${DIM}Credentials: NUT_USER / NUT_PASS env vars, or interactive prompt.${NC}"
    echo
}

for cmd in upsc upscmd; do
    if ! command -v "$cmd" &>/dev/null; then
        err "$cmd not found. Install nut-client."
        exit 1
    fi
done

# Reject remote targets — this script is localhost-only by design.
if [ -n "$UPS" ]; then
    if [[ "$UPS" == *@* ]]; then
        HOST_PART="${UPS##*@}"
        case "$HOST_PART" in
            localhost|127.0.0.1|::1) ;;
            *)
                err "Remote tests are disabled. Run this script on the host with the UPS attached."
                err "Got host: $HOST_PART"
                exit 1
                ;;
        esac
        UPS="${UPS%@*}"
    fi
fi

# Auto-discover the local UPS if not supplied.
if [ -z "$UPS" ]; then
    UPS=$(upsc -l localhost 2>/dev/null | head -n1 || true)
    if [ -z "$UPS" ]; then
        err "No UPS found on localhost. Is nut-server running?"
        exit 1
    fi
fi
TARGET="${UPS}@localhost"

# --- read-only branches first -----------------------------------------------

if [ "$ACTION" = "list" ]; then
    info "INSTCMDs supported by $TARGET:"
    upscmd -l "$TARGET"
    print_hints
    exit 0
fi

if [ "$ACTION" = "status" ]; then
    RESULT=$(upsc "$TARGET" ups.test.result 2>/dev/null || true)
    if [ -z "$RESULT" ]; then
        warn "No test result reported by $TARGET (ups.test.result is empty or unsupported)."
    else
        echo -e "Last test result: ${GREEN}${RESULT}${NC}"
    fi
    print_hints
    exit 0
fi

# --- write branches: need credentials ---------------------------------------

# Pull credentials from env, then nut-credentials.txt (root only), then prompt.
NUT_USER="${NUT_USER:-}"
NUT_PASS="${NUT_PASS:-}"

if { [ -z "$NUT_USER" ] || [ -z "$NUT_PASS" ]; } && [ -r /root/nut-credentials.txt ] && [ "$EUID" -eq 0 ]; then
    : "${NUT_USER:=$(awk -F': *' '/^Admin User:/ {print $2; exit}' /root/nut-credentials.txt)}"
    : "${NUT_PASS:=$(awk -F': *' '/^Admin Pass:/ {print $2; exit}' /root/nut-credentials.txt)}"
fi

if [ -z "$NUT_USER" ]; then
    read -r -p "NUT admin user [admin]: " NUT_USER
    NUT_USER="${NUT_USER:-admin}"
fi
if [ -z "$NUT_PASS" ]; then
    read -r -s -p "NUT password for $NUT_USER: " NUT_PASS
    echo
fi

case "$ACTION" in
    quick) CMD="test.battery.start.quick" ;;
    deep)  CMD="test.battery.start.deep"  ;;
    stop)  CMD="test.battery.stop"        ;;
esac

# Verify this UPS actually supports the requested INSTCMD before firing.
if ! upscmd -l "$TARGET" 2>/dev/null | grep -qx "$CMD"; then
    err "$TARGET does not advertise '$CMD'."
    echo "Supported INSTCMDs on this UPS:"
    upscmd -l "$TARGET" 2>/dev/null | sed 's/^/  /'
    exit 1
fi

# Show what we're about to do.
MODEL=$(upsc "$TARGET" ups.model 2>/dev/null || echo "unknown")
echo
echo "Target:  ${CYAN}${TARGET}${NC} (${MODEL})"
echo "Action:  ${YELLOW}${CMD}${NC}"
echo "User:    ${NUT_USER}"
echo

if [ "$ASSUME_YES" -ne 1 ]; then
    read -r -p "Proceed? [y/N] " CONFIRM
    case "$CONFIRM" in
        y|Y|yes|YES) ;;
        *) info "Cancelled."; print_hints; exit 0 ;;
    esac
fi

info "Sending $CMD ..."
if ! upscmd -u "$NUT_USER" -p "$NUT_PASS" "$TARGET" "$CMD"; then
    err "INSTCMD failed. Check that '$NUT_USER' has 'instcmds = ALL' in /etc/nut/upsd.users."
    exit 1
fi
ok "Command accepted."

# Poll for the result if we started a test.
if [ "$ACTION" = "stop" ]; then
    print_hints
    exit 0
fi

info "Polling ups.test.result (up to 60s)…"
for _ in $(seq 1 12); do
    sleep 5
    RESULT=$(upsc "$TARGET" ups.test.result 2>/dev/null || true)
    [ -z "$RESULT" ] && continue
    echo -e "  ${DIM}…${NC} $RESULT"
    case "$RESULT" in
        *"In progress"*|*"InProgress"*|*Pending*) continue ;;
        *Done*|*OK*|*Passed*|*Failed*|*Aborted*|*Warning*|*"Bad"*)
            ok "Final: $RESULT"
            print_hints
            exit 0
            ;;
    esac
done
warn "Did not see a terminal result within 60s. Check later with: $0 --status"
print_hints
