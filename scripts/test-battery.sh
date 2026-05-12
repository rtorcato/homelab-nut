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
    CYAN='\033[0;36m'; DIM='\033[2m'; BOLD='\033[1m'; NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' CYAN='' DIM='' BOLD='' NC=''
fi

err()  { echo -e "${RED}Error:${NC} $*" >&2; }
warn() { echo -e "${YELLOW}!${NC} $*"; }
ok()   { echo -e "${GREEN}✓${NC} $*"; }
info() { echo -e "${CYAN}»${NC} $*"; }
line() { printf "  ${CYAN}%-14s${NC} %b\n" "$1" "$2"; }
section() { echo; echo -e "${CYAN}─ $* ─${NC}"; }

print_hints() {
    local s
    s=$(basename "$0")
    section "Options"
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
MODEL=$(upsc "$TARGET" ups.model 2>/dev/null || echo "unknown")
MFR=$(upsc "$TARGET" ups.mfr 2>/dev/null || true)

# --- read-only branches -----------------------------------------------------

if [ "$ACTION" = "list" ]; then
    section "UPS"
    line "Target:" "$TARGET"
    [ -n "$MFR$MODEL" ] && line "Model:" "$MFR $MODEL"
    section "Supported Commands"
    # Parse upscmd -l output: "cmd - description" format
    upscmd -l "$TARGET" 2>/dev/null | grep -v '^Instant' | grep ' - ' | \
        while IFS=' - ' read -r cmd desc; do
            printf "  ${CYAN}%-38s${NC} ${DIM}%s${NC}\n" "${cmd// /}" "$desc"
        done
    print_hints
    exit 0
fi

if [ "$ACTION" = "status" ]; then
    RESULT=$(upsc "$TARGET" ups.test.result 2>/dev/null || true)
    section "UPS"
    line "Target:" "$TARGET"
    [ -n "$MFR$MODEL" ] && line "Model:" "$MFR $MODEL"
    section "Last Test Result"
    if [ -z "$RESULT" ]; then
        warn "No result available (ups.test.result is empty or unsupported)."
    else
        case "${RESULT,,}" in
            *"done and passed"*|*ok*|*passed*)
                echo -e "  ${GREEN}${BOLD}${RESULT}${NC}" ;;
            *"in progress"*|*pending*)
                echo -e "  ${YELLOW}${RESULT}${NC}" ;;
            *failed*|*aborted*|*error*|*bad*)
                echo -e "  ${RED}${RESULT}${NC}" ;;
            *)
                echo -e "  ${RESULT}" ;;
        esac
    fi
    print_hints
    exit 0
fi

# --- write branches: need credentials ----------------------------------------

NUT_USER="${NUT_USER:-}"
NUT_PASS="${NUT_PASS:-}"

if { [ -z "$NUT_USER" ] || [ -z "$NUT_PASS" ]; } && [ -r /root/nut-credentials.txt ] && [ "$EUID" -eq 0 ]; then
    : "${NUT_USER:=$(awk -F': *' '/^Admin User:/ {print $2; exit}' /root/nut-credentials.txt)}"
    : "${NUT_PASS:=$(awk -F': *' '/^Admin Pass:/ {print $2; exit}' /root/nut-credentials.txt)}"
fi

if [ -z "$NUT_USER" ] || [ -z "$NUT_PASS" ]; then
    section "Credentials"
    if [ -z "$NUT_USER" ]; then
        echo -en "  ${CYAN}Admin user${NC} [admin]: "
        read -r NUT_USER
        NUT_USER="${NUT_USER:-admin}"
    fi
    if [ -z "$NUT_PASS" ]; then
        echo -en "  ${CYAN}Password${NC} for ${NUT_USER}: "
        read -r -s NUT_PASS
        echo
    fi
fi

case "$ACTION" in
    quick) CMD="test.battery.start.quick" ;;
    deep)  CMD="test.battery.start.deep"  ;;
    stop)  CMD="test.battery.stop"        ;;
esac

# Verify the UPS supports the requested INSTCMD. If --quick isn't supported but
# --deep is, suggest it rather than just bailing with a raw error.
SUPPORTED=$(upscmd -l "$TARGET" 2>/dev/null | grep ' - ' | awk '{print $1}' || true)

if ! echo "$SUPPORTED" | grep -qx "$CMD"; then
    section "Unsupported Command"
    echo -e "  ${RED}${CMD}${NC} is not supported by ${CYAN}${TARGET}${NC}."
    if [ "$ACTION" = "quick" ] && echo "$SUPPORTED" | grep -qx "test.battery.start.deep"; then
        echo
        echo -e "  ${YELLOW}Tip:${NC} this UPS supports a deep test instead:"
        echo -e "  ${DIM}$(basename "$0") --deep${NC}"
    fi
    # Show available battery test commands
    BATT_CMDS=$(echo "$SUPPORTED" | grep '^test\.battery\.' || true)
    if [ -n "$BATT_CMDS" ]; then
        echo
        echo -e "  ${CYAN}Available battery test commands:${NC}"
        echo "$BATT_CMDS" | while read -r c; do
            DESC=$(upscmd -l "$TARGET" 2>/dev/null | grep "^$c " | sed 's/^[^ ]* - //')
            printf "    ${CYAN}%-36s${NC} ${DIM}%s${NC}\n" "$c" "$DESC"
        done
    fi
    print_hints
    exit 1
fi

# Show what we're about to do.
section "UPS"
line "Target:" "$TARGET"
[ -n "$MFR$MODEL" ] && line "Model:" "$MFR $MODEL"
section "Test"
line "Command:" "${YELLOW}${CMD}${NC}"
line "User:" "$NUT_USER"
echo

if [ "$ASSUME_YES" -ne 1 ]; then
    read -r -p "  Proceed? [y/N] " CONFIRM
    case "$CONFIRM" in
        y|Y|yes|YES) ;;
        *) echo; info "Cancelled."; print_hints; exit 0 ;;
    esac
fi

echo
info "Sending ${YELLOW}${CMD}${NC} ..."
if ! upscmd -u "$NUT_USER" -p "$NUT_PASS" "$TARGET" "$CMD"; then
    err "INSTCMD failed. Check that '${NUT_USER}' has 'instcmds = ALL' in /etc/nut/upsd.users."
    exit 1
fi
ok "Command accepted."

if [ "$ACTION" = "stop" ]; then
    print_hints
    exit 0
fi

section "Result"
info "Polling ups.test.result (up to 60s)…"
for _ in $(seq 1 12); do
    sleep 5
    RESULT=$(upsc "$TARGET" ups.test.result 2>/dev/null || true)
    [ -z "$RESULT" ] && continue
    case "${RESULT,,}" in
        *"in progress"*|*"inprogress"*|*pending*)
            echo -e "  ${DIM}… ${RESULT}${NC}"
            continue
            ;;
        *"done and passed"*|*ok*|*passed*)
            ok "${GREEN}${RESULT}${NC}"
            print_hints; exit 0
            ;;
        *failed*|*aborted*|*error*|*bad*)
            warn "${RED}${RESULT}${NC}"
            print_hints; exit 0
            ;;
        *)
            ok "Final: $RESULT"
            print_hints; exit 0
            ;;
    esac
done
warn "No terminal result within 60s. Check later with: $(basename "$0") --status"
print_hints
