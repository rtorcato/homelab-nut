#!/bin/bash
#
# NUT Prometheus Exporter Setup Script (bare-metal, no Docker)
# Installs druggeri/nut_exporter as a systemd service.
# Intended for low-resource hosts (Pi Zero, Pi Zero 2 W) where Docker overhead
# is undesirable. The Docker compose stack in ../docker/compose.yml runs the
# same binary; this script is a drop-in replacement for that container.
#
# Usage:
#   sudo ./setup-exporter.sh [NUT_SERVER] [NUT_USER] [NUT_PASSWORD]
#
# Examples:
#   sudo ./setup-exporter.sh                              # scrape localhost, no auth
#   sudo ./setup-exporter.sh 192.168.1.10 upsmon secret   # scrape remote server
#
# Environment overrides:
#   NUT_EXPORTER_VERSION   pin a release (default: latest from GitHub)
#   NUT_EXPORTER_PORT      listen port (default: 9199)
#   NUT_EXPORTER_ARCH      override arch detection (amd64|arm64|armv7|armv6)
#

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

NUT_SERVER="${1:-localhost}"
NUT_USER="${2:-}"
NUT_PASSWORD="${3:-}"

EXPORTER_PORT="${NUT_EXPORTER_PORT:-9199}"
EXPORTER_USER="nut-exporter"
INSTALL_DIR="/usr/local/bin"
BIN_PATH="${INSTALL_DIR}/nut_exporter"
ENV_FILE="/etc/default/nut-exporter"
UNIT_FILE="/etc/systemd/system/nut-exporter.service"
REPO="DRuggeri/nut_exporter"

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

if [[ $EUID -ne 0 ]]; then
    log_error "This script must be run as root (use sudo)"
    exit 1
fi

# Detect architecture (override with NUT_EXPORTER_ARCH)
detect_arch() {
    if [[ -n "${NUT_EXPORTER_ARCH:-}" ]]; then
        echo "$NUT_EXPORTER_ARCH"
        return
    fi
    local m
    m=$(uname -m)
    case "$m" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        armv7l)         echo "armv7" ;;
        armv6l)         echo "armv6" ;;
        *)
            log_error "Unsupported architecture: $m (set NUT_EXPORTER_ARCH to override)"
            exit 1
            ;;
    esac
}

ARCH=$(detect_arch)
log_info "Detected architecture: $ARCH"

# Required tools
for cmd in curl tar; do
    if ! command -v $cmd &>/dev/null; then
        log_info "Installing missing dependency: $cmd"
        if command -v apt-get &>/dev/null; then
            apt-get update -qq && apt-get install -y "$cmd"
        elif command -v dnf &>/dev/null; then
            dnf install -y "$cmd"
        else
            log_error "Please install '$cmd' manually"
            exit 1
        fi
    fi
done

# Resolve version
VERSION="${NUT_EXPORTER_VERSION:-}"
if [[ -z "$VERSION" ]]; then
    log_info "Looking up latest release from github.com/${REPO}..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -oE '"tag_name": *"[^"]+"' \
        | head -1 \
        | sed -E 's/.*"([^"]+)"$/\1/')
    if [[ -z "$VERSION" ]]; then
        log_error "Could not determine latest version. Set NUT_EXPORTER_VERSION."
        exit 1
    fi
fi
# Strip leading 'v' if present
VERSION_NUM="${VERSION#v}"
log_info "Installing nut_exporter ${VERSION_NUM}"

# Pick asset matching arch. Upstream uses goreleaser with names like:
#   nut_exporter-1.7.1.linux-amd64.tar.gz
#   nut_exporter-1.7.1.linux-arm64.tar.gz
#   nut_exporter-1.7.1.linux-armv6.tar.gz
ASSET="nut_exporter-${VERSION_NUM}.linux-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

log_info "Downloading ${URL}"
if ! curl -fsSL -o "${TMPDIR}/${ASSET}" "$URL"; then
    log_error "Download failed. Asset may not exist for arch '${ARCH}'."
    log_error "Check available assets at https://github.com/${REPO}/releases/tag/${VERSION}"
    exit 1
fi

log_info "Extracting binary..."
tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

# Locate the binary (release layout has historically varied)
EXTRACTED_BIN=$(find "$TMPDIR" -type f -name 'nut_exporter' -perm -u+x | head -1)
if [[ -z "$EXTRACTED_BIN" ]]; then
    EXTRACTED_BIN=$(find "$TMPDIR" -type f -name 'nut_exporter' | head -1)
fi
if [[ -z "$EXTRACTED_BIN" ]]; then
    log_error "Could not find nut_exporter binary inside ${ASSET}"
    exit 1
fi

# Stop existing service before replacing the binary
if systemctl is-active --quiet nut-exporter 2>/dev/null; then
    log_info "Stopping existing nut-exporter service..."
    systemctl stop nut-exporter
fi

log_info "Installing binary to ${BIN_PATH}"
install -m 0755 "$EXTRACTED_BIN" "$BIN_PATH"

# Create dedicated unprivileged user
if ! id -u "$EXPORTER_USER" &>/dev/null; then
    log_info "Creating system user '${EXPORTER_USER}'"
    useradd --system --no-create-home --shell /usr/sbin/nologin "$EXPORTER_USER"
fi

# Write env file (credentials live here, not in the unit file)
log_info "Writing ${ENV_FILE}"
umask 077
cat > "$ENV_FILE" << EOF
# nut_exporter configuration
# Reloaded by: systemctl restart nut-exporter

NUT_EXPORTER_SERVER=${NUT_SERVER}
NUT_EXPORTER_USERNAME=${NUT_USER}
NUT_EXPORTER_PASSWORD=${NUT_PASSWORD}
NUT_EXPORTER_LISTEN_ADDRESS=:${EXPORTER_PORT}
EOF
chown root:"$EXPORTER_USER" "$ENV_FILE"
chmod 640 "$ENV_FILE"
umask 022

# Build flag list. Auth flags are only set when a username is provided so the
# exporter can talk to NUT servers that don't require authentication.
EXEC_FLAGS='--nut.server=${NUT_EXPORTER_SERVER} --web.listen-address=${NUT_EXPORTER_LISTEN_ADDRESS}'
if [[ -n "$NUT_USER" ]]; then
    EXEC_FLAGS="${EXEC_FLAGS} "'--nut.username=${NUT_EXPORTER_USERNAME}'
fi

log_info "Writing systemd unit ${UNIT_FILE}"
cat > "$UNIT_FILE" << EOF
[Unit]
Description=NUT Prometheus Exporter
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${EXPORTER_USER}
Group=${EXPORTER_USER}
EnvironmentFile=${ENV_FILE}
ExecStart=${BIN_PATH} ${EXEC_FLAGS}
Restart=on-failure
RestartSec=5

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictNamespaces=true
RestrictRealtime=true
LockPersonality=true

[Install]
WantedBy=multi-user.target
EOF

log_info "Reloading systemd and starting service..."
systemctl daemon-reload
systemctl enable nut-exporter.service
systemctl restart nut-exporter.service

# Verify
sleep 2
if systemctl is-active --quiet nut-exporter; then
    log_info "✓ nut-exporter is running"
else
    log_error "nut-exporter failed to start. Check: journalctl -u nut-exporter -n 50"
    exit 1
fi

# Smoke-test the metrics endpoint
log_info "Testing metrics endpoint on :${EXPORTER_PORT}..."
sleep 1
if command -v curl &>/dev/null; then
    if curl -fsS --max-time 5 "http://127.0.0.1:${EXPORTER_PORT}/ups_metrics?ups=${4:-}" >/dev/null 2>&1 \
       || curl -fsS --max-time 5 "http://127.0.0.1:${EXPORTER_PORT}/metrics" >/dev/null 2>&1; then
        log_info "✓ Exporter is responding"
    else
        log_warn "Exporter is up but metrics request failed."
        log_warn "This is normal if NUT server '${NUT_SERVER}' is unreachable or no UPS name was queried."
    fi
fi

HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[[ -z "$HOST_IP" ]] && HOST_IP="<this-host>"

echo ""
echo "=============================================="
echo -e "${GREEN}NUT Exporter Setup Complete!${NC}"
echo "=============================================="
echo ""
echo "Version:        ${VERSION_NUM}"
echo "Architecture:   ${ARCH}"
echo "Listen address: :${EXPORTER_PORT}"
echo "NUT server:     ${NUT_SERVER}"
echo "Auth user:      ${NUT_USER:-<none>}"
echo ""
echo "Metrics URL:    http://${HOST_IP}:${EXPORTER_PORT}/ups_metrics?ups=<UPS_NAME>"
echo "Service logs:   journalctl -u nut-exporter -f"
echo "Edit config:    sudo ${EDITOR:-nano} ${ENV_FILE} && sudo systemctl restart nut-exporter"
echo ""
echo "Add to Prometheus scrape config:"
echo "  - job_name: nut"
echo "    metrics_path: /ups_metrics"
echo "    params:"
echo "      ups: [<UPS_NAME>]"
echo "    static_configs:"
echo "      - targets: ['${HOST_IP}:${EXPORTER_PORT}']"
echo ""
