#!/usr/bin/env bash
#
# UPS Status Script
# Quick status check for NUT-monitored UPS — run on the Pi
#
# Usage: ./ups-status.sh [UPS@HOST]
#
set -euo pipefail

UPS="${1:-myups@localhost}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

# Check if upsc is available
if ! command -v upsc &>/dev/null; then
    echo "Error: upsc command not found. Is NUT installed?"
    exit 1
fi

# Get all data
if ! DATA=$(upsc "$UPS" 2>/dev/null); then
    echo -e "${RED}Error: Cannot connect to $UPS${NC}"
    exit 1
fi

# Extract values
get_val() {
    echo "$DATA" | grep "^$1:" | cut -d: -f2- | sed 's/^ *//' || true
}

STATUS=$(get_val "ups.status")
CHARGE=$(get_val "battery.charge")
RUNTIME=$(get_val "battery.runtime")
LOAD=$(get_val "ups.load")
INPUT_V=$(get_val "input.voltage")
OUTPUT_V=$(get_val "output.voltage")
MODEL=$(get_val "ups.model")
MFR=$(get_val "ups.mfr")

# Calculate runtime in minutes
RUNTIME_MIN=""
if [ -n "$RUNTIME" ]; then
    RUNTIME_MIN=$((RUNTIME / 60))
fi

# Status color
case "$STATUS" in
    *"OL"*)
        STATUS_COLOR=$GREEN
        STATUS_ICON="●"
        STATUS_TEXT="Online"
        ;;
    *"OB"*)
        STATUS_COLOR=$YELLOW
        STATUS_ICON="⚡"
        STATUS_TEXT="On Battery"
        ;;
    *"LB"*)
        STATUS_COLOR=$RED
        STATUS_ICON="⚠"
        STATUS_TEXT="Low Battery"
        ;;
    *)
        STATUS_COLOR=$CYAN
        STATUS_ICON="?"
        STATUS_TEXT="$STATUS"
        ;;
esac

# Battery color
BATT_COLOR=$NC
if [ -n "$CHARGE" ]; then
    if [ "$CHARGE" -ge 80 ]; then
        BATT_COLOR=$GREEN
    elif [ "$CHARGE" -ge 40 ]; then
        BATT_COLOR=$YELLOW
    else
        BATT_COLOR=$RED
    fi
fi

# Output
echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║${NC}  UPS Status:  ${STATUS_COLOR}${STATUS_ICON} ${STATUS_TEXT}${NC}"
echo -e "${CYAN}╠══════════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  UPS:         ${UPS}"
[ -n "$MFR" ] && echo -e "${CYAN}║${NC}  Model:       ${MFR} ${MODEL}"
echo -e "${CYAN}╠══════════════════════════════════════════════╣${NC}"
[ -n "$CHARGE" ]     && echo -e "${CYAN}║${NC}  Battery:     ${BATT_COLOR}${CHARGE}%${NC}"
[ -n "$RUNTIME_MIN" ] && echo -e "${CYAN}║${NC}  Runtime:     ${RUNTIME_MIN} minutes"
[ -n "$LOAD" ]       && echo -e "${CYAN}║${NC}  Load:        ${LOAD}%"
echo -e "${CYAN}╠══════════════════════════════════════════════╣${NC}"
[ -n "$INPUT_V" ]  && echo -e "${CYAN}║${NC}  Input:       ${INPUT_V}V"
[ -n "$OUTPUT_V" ] && echo -e "${CYAN}║${NC}  Output:      ${OUTPUT_V}V"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""
