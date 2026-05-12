#!/bin/bash
#
# UPS Status (via nut-exporter HTTP)
# Pulls UPS metrics from a nut_exporter endpoint and renders them like
# ups-status.sh. Works from any machine with curl — no `upsc` needed.
#
# Usage:
#   ./exporter-status.sh [EXPORTER_URL] [UPS_NAME]
#
# Examples:
#   ./exporter-status.sh http://192.0.2.10:9199 myups
#   EXPORTER_URL=http://pi:9199 ./exporter-status.sh
#
# Flags:
#   --raw     dump all metric lines (no parsing) and exit
#   --json    emit a JSON object with the extracted fields
#

set -u

EXPORTER_URL="${1:-${EXPORTER_URL:-http://localhost:9199}}"
UPS_NAME="${2:-${UPS_NAME:-myups}}"
MODE="pretty"

for arg in "$@"; do
    case "$arg" in
        --raw)  MODE="raw" ;;
        --json) MODE="json" ;;
    esac
done

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

if ! command -v curl &>/dev/null; then
    echo "Error: curl is required" >&2
    exit 1
fi

URL="${EXPORTER_URL%/}/ups_metrics?ups=${UPS_NAME}"
DATA=$(curl -fsS --max-time 5 "$URL") || {
    echo -e "${RED}Error: Cannot fetch $URL${NC}" >&2
    exit 1
}

if [[ "$MODE" == "raw" ]]; then
    echo "$DATA"
    exit 0
fi

# Extract the numeric value of a top-level (unlabeled) metric, or the first
# matching labelled series. Returns empty string if not present.
metric_value() {
    local name=$1
    awk -v n="$name" '
        $0 ~ "^"n"( |\\{)" {
            v = $NF
            if (v == "NaN" || v == "+Inf" || v == "-Inf") next
            print v
            exit
        }
    ' <<<"$DATA"
}

# Extract a label value from any series of a given metric.
label_value() {
    local metric=$1 label=$2
    awk -v m="$metric" -v l="$label" '
        $0 ~ "^"m"\\{" {
            if (match($0, l"=\"[^\"]*\"")) {
                v = substr($0, RSTART, RLENGTH)
                sub(l"=\"", "", v)
                sub("\"$", "", v)
                print v
                exit
            }
        }
    ' <<<"$DATA"
}

# Active status flags (each flag has value 1 when set). The exporter emits:
#   network_ups_tools_ups_status{flag="OL",...} 1
active_flags() {
    awk '
        /^network_ups_tools_ups_status\{/ {
            if ($NF == 1 && match($0, /flag="[^"]+"/)) {
                f = substr($0, RSTART+6, RLENGTH-7)
                printf "%s ", f
            }
        }
    ' <<<"$DATA"
}

CHARGE=$(metric_value "network_ups_tools_battery_charge")
RUNTIME=$(metric_value "network_ups_tools_battery_runtime")
BATT_V=$(metric_value  "network_ups_tools_battery_voltage")
LOAD=$(metric_value    "network_ups_tools_ups_load")
INPUT_V=$(metric_value "network_ups_tools_input_voltage")
OUTPUT_V=$(metric_value "network_ups_tools_output_voltage")
TEMP=$(metric_value    "network_ups_tools_ups_temperature")

# Device info — labels live on the info gauge (label name varies by exporter
# version, so check a couple of common ones).
MFR=$(label_value     "network_ups_tools_ups" "device_mfr")
[[ -z "$MFR" ]] && MFR=$(label_value "network_ups_tools_ups" "ups_mfr")
MODEL=$(label_value   "network_ups_tools_ups" "device_model")
[[ -z "$MODEL" ]] && MODEL=$(label_value "network_ups_tools_ups" "ups_model")
SERIAL=$(label_value  "network_ups_tools_ups" "device_serial")
[[ -z "$SERIAL" ]] && SERIAL=$(label_value "network_ups_tools_ups" "ups_serial")

FLAGS=$(active_flags)
FLAGS="${FLAGS% }"   # trim trailing space

# Pick primary status from the flag set
if   [[ " $FLAGS " == *" LB "* ]]; then STATUS_FLAG="LB"
elif [[ " $FLAGS " == *" OB "* ]]; then STATUS_FLAG="OB"
elif [[ " $FLAGS " == *" OL "* ]]; then STATUS_FLAG="OL"
else STATUS_FLAG="${FLAGS%% *}"
fi

case "$STATUS_FLAG" in
    OL) STATUS_COLOR=$GREEN;  STATUS_ICON="●"; STATUS_TEXT="Online" ;;
    OB) STATUS_COLOR=$YELLOW; STATUS_ICON="⚡"; STATUS_TEXT="On Battery" ;;
    LB) STATUS_COLOR=$RED;    STATUS_ICON="⚠"; STATUS_TEXT="Low Battery" ;;
    "") STATUS_COLOR=$CYAN;   STATUS_ICON="?"; STATUS_TEXT="Unknown" ;;
    *)  STATUS_COLOR=$CYAN;   STATUS_ICON="?"; STATUS_TEXT="$STATUS_FLAG" ;;
esac

# Convert numerics to ints where it makes sense for display
to_int() { printf '%.0f' "${1:-0}" 2>/dev/null || echo "$1"; }
CHARGE_I=""; LOAD_I=""; RUNTIME_MIN=""
[[ -n "$CHARGE"  ]] && CHARGE_I=$(to_int "$CHARGE")
[[ -n "$LOAD"    ]] && LOAD_I=$(to_int "$LOAD")
[[ -n "$RUNTIME" ]] && RUNTIME_MIN=$(( $(to_int "$RUNTIME") / 60 ))

if [[ "$MODE" == "json" ]]; then
    esc() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }
    cat <<EOF
{
  "ups": "$(esc "$UPS_NAME")",
  "exporter": "$(esc "$EXPORTER_URL")",
  "status": "$(esc "$STATUS_FLAG")",
  "status_text": "$(esc "$STATUS_TEXT")",
  "flags": "$(esc "$FLAGS")",
  "battery_charge_pct": ${CHARGE_I:-null},
  "battery_runtime_sec": ${RUNTIME:-null},
  "battery_voltage": ${BATT_V:-null},
  "load_pct": ${LOAD_I:-null},
  "input_voltage": ${INPUT_V:-null},
  "output_voltage": ${OUTPUT_V:-null},
  "temperature": ${TEMP:-null},
  "mfr": "$(esc "$MFR")",
  "model": "$(esc "$MODEL")",
  "serial": "$(esc "$SERIAL")"
}
EOF
    exit 0
fi

# Battery color
BATT_COLOR=$CYAN
if [[ -n "$CHARGE_I" ]]; then
    if   (( CHARGE_I >= 80 )); then BATT_COLOR=$GREEN
    elif (( CHARGE_I >= 40 )); then BATT_COLOR=$YELLOW
    else                            BATT_COLOR=$RED
    fi
fi

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║${NC}            UPS Status: ${STATUS_COLOR}$STATUS_ICON $STATUS_TEXT${NC}"
echo -e "${CYAN}╠══════════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  UPS:        ${UPS_NAME}"
echo -e "${CYAN}║${NC}  Exporter:   ${EXPORTER_URL}"
[[ -n "$MFR$MODEL" ]] && echo -e "${CYAN}║${NC}  Model:      ${MFR} ${MODEL}"
[[ -n "$SERIAL" ]]    && echo -e "${CYAN}║${NC}  Serial:     ${SERIAL}"
[[ -n "$FLAGS" ]]     && echo -e "${CYAN}║${NC}  Flags:      ${FLAGS}"
echo -e "${CYAN}╠══════════════════════════════════════════════╣${NC}"
[[ -n "$CHARGE_I" ]]    && echo -e "${CYAN}║${NC}  Battery:    ${BATT_COLOR}${CHARGE_I}%${NC}"
[[ -n "$RUNTIME_MIN" ]] && echo -e "${CYAN}║${NC}  Runtime:    ${RUNTIME_MIN} minutes"
[[ -n "$LOAD_I" ]]      && echo -e "${CYAN}║${NC}  Load:       ${LOAD_I}%"
[[ -n "$BATT_V" ]]      && echo -e "${CYAN}║${NC}  Batt Volt:  ${BATT_V}V"
[[ -n "$TEMP" ]]        && echo -e "${CYAN}║${NC}  Temp:       ${TEMP}°C"
echo -e "${CYAN}╠══════════════════════════════════════════════╣${NC}"
[[ -n "$INPUT_V" ]]  && echo -e "${CYAN}║${NC}  Input:      ${INPUT_V}V"
[[ -n "$OUTPUT_V" ]] && echo -e "${CYAN}║${NC}  Output:     ${OUTPUT_V}V"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""
echo "Tip: re-run with --raw to dump every metric the exporter publishes,"
echo "     or --json to emit machine-readable output."
echo ""
