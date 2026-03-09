#!/bin/bash
#
# NUT Client Setup Script
# Installs and configures NUT client on Debian/Ubuntu systems
#
# Usage: sudo ./setup-client.sh <SERVER_IP> <UPS_NAME> <PASSWORD>
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check arguments
if [ $# -lt 3 ]; then
    echo "Usage: $0 <SERVER_IP> <UPS_NAME> <PASSWORD>"
    echo ""
    echo "Example: $0 192.168.1.10 myups secretpassword"
    echo ""
    echo "Get these values from your NUT server setup"
    exit 1
fi

SERVER_IP="$1"
UPS_NAME="$2"
PASSWORD="$3"
REMOTE_USER="upsmon_remote"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    log_error "This script must be run as root (use sudo)"
    exit 1
fi

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
else
    log_error "Cannot detect OS"
    exit 1
fi

log_info "Detected OS: $OS"
log_info "Setting up NUT client..."
log_info "Server: $SERVER_IP, UPS: $UPS_NAME"

# Install NUT client
log_info "Installing NUT client..."
case $OS in
    ubuntu|debian)
        apt-get update
        apt-get install -y nut-client
        ;;
    centos|rhel|fedora)
        dnf install -y nut-client
        ;;
    alpine)
        apk add nut
        ;;
    *)
        log_error "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Test connection to server first
log_info "Testing connection to NUT server..."
if ! nc -zv $SERVER_IP 3493 2>&1 | grep -q succeeded; then
    log_warn "Cannot connect to $SERVER_IP:3493"
    log_warn "Make sure the NUT server is running and firewall allows port 3493"
fi

# Create nut.conf
log_info "Configuring NUT mode..."
cat > /etc/nut/nut.conf << EOF
# NUT Client Mode
MODE=netclient
EOF

# Create upsmon.conf
log_info "Configuring UPS monitor..."
cat > /etc/nut/upsmon.conf << EOF
# UPS Monitor Configuration (Client Mode)
# Monitors remote NUT server

MONITOR $UPS_NAME@$SERVER_IP 1 $REMOTE_USER $PASSWORD slave

MINSUPPLIES 1
SHUTDOWNCMD "/sbin/shutdown -h +0"
POLLFREQ 5
POLLFREQALERT 5
DEADTIME 25
FINALDELAY 5

# Notifications
NOTIFYFLAG ONLINE SYSLOG+WALL
NOTIFYFLAG ONBATT SYSLOG+WALL
NOTIFYFLAG LOWBATT SYSLOG+WALL
NOTIFYFLAG FSD SYSLOG+WALL
NOTIFYFLAG COMMOK SYSLOG+WALL
NOTIFYFLAG COMMBAD SYSLOG+WALL
NOTIFYFLAG SHUTDOWN SYSLOG+WALL
NOTIFYFLAG REPLBATT SYSLOG+WALL
NOTIFYFLAG NOCOMM SYSLOG+WALL

RUN_AS_USER root
EOF

# Fix permissions
log_info "Setting file permissions..."
chown root:nut /etc/nut/*.conf 2>/dev/null || chown root:root /etc/nut/*.conf
chmod 640 /etc/nut/*.conf

# Start service
log_info "Starting NUT monitor service..."
systemctl daemon-reload
systemctl enable nut-monitor
systemctl restart nut-monitor

# Test connection
log_info "Testing UPS connection..."
sleep 2
if upsc $UPS_NAME@$SERVER_IP &>/dev/null; then
    log_info "✓ Connected to UPS successfully!"
    echo ""
    echo "UPS Status:"
    echo "----------------------------------------------"
    upsc $UPS_NAME@$SERVER_IP | grep -E "battery.charge|battery.runtime|ups.status|ups.load"
else
    log_error "Could not connect to UPS at $SERVER_IP"
    log_warn "Check server IP, UPS name, and credentials"
    log_warn "Run: upsc $UPS_NAME@$SERVER_IP  to debug"
    exit 1
fi

echo ""
echo "=============================================="
echo -e "${GREEN}NUT Client Setup Complete!${NC}"
echo "=============================================="
echo ""
echo "Monitoring: $UPS_NAME@$SERVER_IP"
echo ""
echo "Test command: upsc $UPS_NAME@$SERVER_IP"
echo ""
echo "This system will now shut down gracefully when"
echo "the UPS battery is low."
echo ""
