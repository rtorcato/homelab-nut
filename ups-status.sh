#!/usr/bin/env bash
#
# UPS Status Script
# Quick status check for NUT-monitored UPS — run on the Pi
#
# Usage: ./ups-status.sh [UPS@HOST] [--watch[=SECONDS]] [--json] [--verbose]
#
set -euo pipefail

UPS=""
WATCH=0
WATCH_INTERVAL=2
JSON=0
VERBOSE=0

for arg in "$@"; do
    case "$arg" in
        --watch)        WATCH=1 ;;
        --watch=*)      WATCH=1; WATCH_INTERVAL="${arg#*=}" ;;
        --json|-j)      JSON=1 ;;
        --verbose|-v)   VERBOSE=1 ;;
        -h|--help)
            sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        -*)
            echo "Unknown option: $arg" >&2
            exit 2
            ;;
        *)              UPS="$arg" ;;
    esac
done

# Colors (disabled when not a tty or in JSON mode)
if [ -t 1 ] && [ "$JSON" -eq 0 ]; then
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    RED='\033[0;31m'
    CYAN='\033[0;36m'
    DIM='\033[2m'
    NC='\033[0m'
else
    GREEN='' YELLOW='' RED='' CYAN='' DIM='' NC=''
fi

if ! command -v upsc &>/dev/null; then
    echo "Error: upsc command not found. Is NUT installed?" >&2
    exit 1
fi

# If no UPS specified, auto-discover the first UPS on localhost so the
# script works on any host without remembering its UPS name.
if [ -z "$UPS" ]; then
    FIRST_UPS=$(upsc -l localhost 2>/dev/null | head -n1)
    if [ -z "$FIRST_UPS" ]; then
        echo "Error: No UPS found on localhost. Is nut-server running?" >&2
        echo "Pass UPS explicitly, e.g.: $0 myups@192.168.1.10" >&2
        exit 1
    fi
    UPS="${FIRST_UPS}@localhost"
fi

is_int() { [[ "${1:-}" =~ ^-?[0-9]+$ ]]; }
is_num() { [[ "${1:-}" =~ ^-?[0-9]+(\.[0-9]+)?$ ]]; }

get_val() {
    local key="$1"
    awk -F': ' -v k="$key" '$1==k {sub(/^[^:]*: */,""); print; exit}' <<<"$DATA"
}

# Decode possibly multi-flag ups.status into human strings + a severity.
# Severity drives the headline color: 0=ok, 1=warn, 2=critical, 3=unknown.
decode_status() {
    local raw="$1"
    local -a flags=()
    local severity=0
    local icon="●"
    [ -z "$raw" ] && { echo "3|?|Unknown"; return; }

    for f in $raw; do
        case "$f" in
            OL)      flags+=("Online") ;;
            OB)      flags+=("On Battery"); severity=$(( severity > 1 ? severity : 1 )); icon="⚡" ;;
            LB)      flags+=("Low Battery"); severity=2; icon="⚠" ;;
            HB)      flags+=("High Battery"); severity=$(( severity > 1 ? severity : 1 )) ;;
            RB)      flags+=("Replace Battery"); severity=2 ;;
            CHRG)    flags+=("Charging") ;;
            DISCHRG) flags+=("Discharging") ;;
            BYPASS)  flags+=("Bypass"); severity=$(( severity > 1 ? severity : 1 )) ;;
            CAL)     flags+=("Calibrating") ;;
            OFF)     flags+=("Off"); severity=2 ;;
            OVER)    flags+=("Overloaded"); severity=2 ;;
            TRIM)    flags+=("Trim (SmartBoost)") ;;
            BOOST)   flags+=("Boost") ;;
            FSD)     flags+=("Forced Shutdown"); severity=2; icon="⚠" ;;
            ALARM)   flags+=("Alarm"); severity=2 ;;
            *)       flags+=("$f") ;;
        esac
    done

    local joined
    joined=$(IFS=', '; echo "${flags[*]}")
    echo "$severity|$icon|$joined"
}

render() {
    if ! DATA=$(upsc "$UPS" 2>/dev/null); then
        echo -e "${RED}Error: Cannot connect to $UPS${NC}" >&2
        return 1
    fi

    if [ "$VERBOSE" -eq 1 ]; then
        echo "$DATA"
        return 0
    fi

    if [ "$JSON" -eq 1 ]; then
        # Emit raw key/values as JSON
        awk -F': ' 'BEGIN{print "{"} NR>1{printf ",\n"} {
            k=$1; sub(/^[^:]*: */,"",$0); v=$0;
            gsub(/\\/,"\\\\",v); gsub(/"/,"\\\"",v);
            printf "  \"%s\": \"%s\"", k, v
        } END{print "\n}"}' <<<"$DATA"
        return 0
    fi

    local STATUS CHARGE RUNTIME LOAD INPUT_V OUTPUT_V MODEL MFR
    local BATT_V BATT_V_NOM BATT_TYPE BATT_DATE BATT_LOW
    local REAL_W REAL_W_NOM TEMP IN_F OUT_F IN_V_NOM
    local TEST_RESULT BEEPER FIRMWARE SERIAL
    local IN_LOW IN_HIGH SHUTDOWN_TIMER

    STATUS=$(get_val "ups.status")
    CHARGE=$(get_val "battery.charge")
    RUNTIME=$(get_val "battery.runtime")
    LOAD=$(get_val "ups.load")
    INPUT_V=$(get_val "input.voltage")
    OUTPUT_V=$(get_val "output.voltage")
    MODEL=$(get_val "ups.model")
    MFR=$(get_val "ups.mfr")
    BATT_V=$(get_val "battery.voltage")
    BATT_V_NOM=$(get_val "battery.voltage.nominal")
    BATT_TYPE=$(get_val "battery.type")
    BATT_DATE=$(get_val "battery.date")
    [ -z "$BATT_DATE" ] && BATT_DATE=$(get_val "battery.mfr.date")
    BATT_LOW=$(get_val "battery.charge.low")
    REAL_W=$(get_val "ups.realpower")
    REAL_W_NOM=$(get_val "ups.realpower.nominal")
    TEMP=$(get_val "ups.temperature")
    IN_F=$(get_val "input.frequency")
    OUT_F=$(get_val "output.frequency")
    IN_V_NOM=$(get_val "input.voltage.nominal")
    IN_LOW=$(get_val "input.transfer.low")
    IN_HIGH=$(get_val "input.transfer.high")
    TEST_RESULT=$(get_val "ups.test.result")
    BEEPER=$(get_val "ups.beeper.status")
    FIRMWARE=$(get_val "ups.firmware")
    SERIAL=$(get_val "ups.serial")
    SHUTDOWN_TIMER=$(get_val "ups.timer.shutdown")

    # Runtime in minutes (handles missing/non-numeric gracefully)
    local RUNTIME_DISPLAY=""
    if is_int "$RUNTIME"; then
        local mins=$((RUNTIME / 60))
        local secs=$((RUNTIME % 60))
        RUNTIME_DISPLAY=$(printf "%dm %02ds" "$mins" "$secs")
    fi

    # Status decode
    local sev icon text
    IFS='|' read -r sev icon text < <(decode_status "$STATUS")
    local STATUS_COLOR
    case "$sev" in
        0) STATUS_COLOR=$GREEN ;;
        1) STATUS_COLOR=$YELLOW ;;
        2) STATUS_COLOR=$RED ;;
        *) STATUS_COLOR=$CYAN ;;
    esac

    # Battery color
    local BATT_COLOR=$NC
    if is_int "$CHARGE"; then
        if [ "$CHARGE" -ge 80 ]; then      BATT_COLOR=$GREEN
        elif [ "$CHARGE" -ge 40 ]; then    BATT_COLOR=$YELLOW
        else                               BATT_COLOR=$RED
        fi
    fi

    # Load color
    local LOAD_COLOR=$NC
    if is_num "$LOAD"; then
        local load_int=${LOAD%.*}
        if   [ "$load_int" -ge 80 ]; then LOAD_COLOR=$RED
        elif [ "$load_int" -ge 50 ]; then LOAD_COLOR=$YELLOW
        else LOAD_COLOR=$GREEN
        fi
    fi

    # Temperature color (rough thresholds)
    local TEMP_COLOR=$NC
    if is_num "$TEMP"; then
        local t=${TEMP%.*}
        if   [ "$t" -ge 45 ]; then TEMP_COLOR=$RED
        elif [ "$t" -ge 35 ]; then TEMP_COLOR=$YELLOW
        else TEMP_COLOR=$GREEN
        fi
    fi

    # Test result color
    local TEST_COLOR=$NC
    case "${TEST_RESULT,,}" in
        *"done and passed"*|*"ok"*|*passed*) TEST_COLOR=$GREEN ;;
        *"in progress"*)                     TEST_COLOR=$YELLOW ;;
        *failed*|*aborted*|*error*)          TEST_COLOR=$RED ;;
    esac

    local line
    line() { printf '  %-13s %b\n' "$1" "$2"; }

    echo
    echo -e "${CYAN}━━━ UPS Status: ${STATUS_COLOR}${icon} ${text}${CYAN} ━━━${NC}"
    echo
    line "Host:"     "$UPS"
    [ -n "$MFR$MODEL" ] && line "Model:" "$MFR $MODEL"
    [ -n "$SERIAL" ]    && line "Serial:"   "${DIM}${SERIAL}${NC}"
    [ -n "$FIRMWARE" ] && line "Firmware:" "${DIM}${FIRMWARE}${NC}"
    [ -n "$SHUTDOWN_TIMER" ] && is_int "$SHUTDOWN_TIMER" && [ "$SHUTDOWN_TIMER" -ge 0 ] && \
        line "Shutdown in:" "${RED}${SHUTDOWN_TIMER}s${NC}"

    echo
    echo -e "${CYAN}─ Battery ─${NC}"
    [ -n "$CHARGE" ]    && line "Charge:"   "${BATT_COLOR}${CHARGE}%${NC}$([ -n "$BATT_LOW" ] && echo "${DIM} (low at ${BATT_LOW}%)${NC}")"
    [ -n "$RUNTIME_DISPLAY" ] && line "Runtime:"  "$RUNTIME_DISPLAY"
    if [ -n "$BATT_V" ]; then
        local bv="${BATT_V}V"
        [ -n "$BATT_V_NOM" ] && bv="$bv ${DIM}(nominal ${BATT_V_NOM}V)${NC}"
        line "Voltage:" "$bv"
    fi
    [ -n "$BATT_TYPE" ] && line "Type:" "$BATT_TYPE"
    [ -n "$BATT_DATE" ] && line "Installed:" "$BATT_DATE"
    [ -n "$TEST_RESULT" ] && line "Self-test:" "${TEST_COLOR}${TEST_RESULT}${NC}"

    echo
    echo -e "${CYAN}─ Power ─${NC}"
    [ -n "$LOAD" ]   && line "Load:" "${LOAD_COLOR}${LOAD}%${NC}"
    if [ -n "$REAL_W" ]; then
        local rw="${REAL_W}W"
        [ -n "$REAL_W_NOM" ] && rw="$rw ${DIM}of ${REAL_W_NOM}W${NC}"
        line "Real power:" "$rw"
    fi
    if [ -n "$INPUT_V" ]; then
        local iv="${INPUT_V}V"
        [ -n "$IN_V_NOM" ] && iv="$iv ${DIM}(nominal ${IN_V_NOM}V)${NC}"
        line "Input:" "$iv"
    fi
    [ -n "$OUTPUT_V" ] && line "Output:" "${OUTPUT_V}V"
    [ -n "$IN_F" ]     && line "In freq:" "${IN_F}Hz"
    [ -n "$OUT_F" ] && [ "$OUT_F" != "$IN_F" ] && line "Out freq:" "${OUT_F}Hz"
    [ -n "$IN_LOW$IN_HIGH" ] && line "Transfer:" "${DIM}${IN_LOW:-?}V – ${IN_HIGH:-?}V${NC}"

    if [ -n "$TEMP" ] || [ -n "$BEEPER" ]; then
        echo
        echo -e "${CYAN}─ Other ─${NC}"
        [ -n "$TEMP" ]   && line "Temp:"   "${TEMP_COLOR}${TEMP}°C${NC}"
        [ -n "$BEEPER" ] && line "Beeper:" "$BEEPER"
    fi
    echo
    print_hints
}

print_hints() {
    local script
    script=$(basename "$0")
    echo -e "${CYAN}─ Options ─${NC}"
    echo -e "  ${DIM}$script [UPS@HOST]            target a specific UPS (default: myups@localhost)${NC}"
    echo -e "  ${DIM}$script --watch[=SECONDS]    auto-refresh (default 2s)${NC}"
    echo -e "  ${DIM}$script --json | -j          raw key/value JSON dump${NC}"
    echo -e "  ${DIM}$script --verbose | -v       full upsc output${NC}"
    echo -e "  ${DIM}$script --help | -h          show usage${NC}"
    echo
}

if [ "$WATCH" -eq 1 ]; then
    if ! is_num "$WATCH_INTERVAL" || [ "${WATCH_INTERVAL%.*}" -lt 1 ]; then
        echo "Invalid --watch interval: $WATCH_INTERVAL" >&2
        exit 2
    fi
    trap 'echo; exit 0' INT
    while true; do
        clear
        echo -e "${DIM}Refreshing every ${WATCH_INTERVAL}s — Ctrl-C to exit${NC}"
        render || true
        sleep "$WATCH_INTERVAL"
    done
else
    render
fi
