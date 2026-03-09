# NUT Client Setup Guide

This guide covers configuring NUT client on machines that need to monitor a remote UPS and shut down gracefully during power events.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Starting the Client](#starting-the-client)
- [Platform-Specific Setup](#platform-specific-setup)
- [Testing](#testing)
- [Notifications](#notifications)
- [Troubleshooting](#troubleshooting)

## Overview

NUT clients connect to a NUT server over the network to:
- Monitor UPS status
- Receive power event notifications
- Execute graceful shutdowns when battery is low

```
┌─────────────┐          ┌─────────────┐
│ NUT Server  │◀────────▶│ NUT Client  │
│ 192.168.1.10│  TCP 3493│ 192.168.1.20│
└─────────────┘          └─────────────┘
```

## Prerequisites

- NUT server already configured and running (see [Server Setup](server-setup.md))
- Network connectivity to NUT server on port 3493
- Remote user credentials from the server's `upsd.users`
- Root/sudo access on client machine

## Installation

### Debian/Ubuntu

```bash
sudo apt update
sudo apt install nut-client
```

### RHEL/CentOS/Fedora

```bash
sudo dnf install nut-client
```

### Alpine Linux

```bash
apk add nut
```

### macOS (Homebrew)

```bash
brew install nut
```

## Configuration

### Step 1: Set NUT Mode

Edit `/etc/nut/nut.conf`:

```bash
sudo nano /etc/nut/nut.conf
```

```ini
MODE=netclient
```

### Step 2: Configure upsmon

Edit `/etc/nut/upsmon.conf`:

```bash
sudo nano /etc/nut/upsmon.conf
```

```ini
# Monitor configuration
# Format: MONITOR <ups>@<server> <powervalue> <user> <password> slave
MONITOR myups@192.168.1.10 1 upsmon_remote your_remote_password slave

# Minimum supplies needed (usually 1)
MINSUPPLIES 1

# Shutdown command
SHUTDOWNCMD "/sbin/shutdown -h +0"

# Polling frequency (seconds)
POLLFREQ 5

# Poll frequency when on battery
POLLFREQALERT 5

# How long to wait before declaring UPS dead
DEADTIME 15

# Final delay before shutdown (seconds)
FINALDELAY 5

# Notification settings
NOTIFYFLAG ONLINE SYSLOG+WALL
NOTIFYFLAG ONBATT SYSLOG+WALL
NOTIFYFLAG LOWBATT SYSLOG+WALL
NOTIFYFLAG FSD SYSLOG+WALL
NOTIFYFLAG COMMOK SYSLOG+WALL
NOTIFYFLAG COMMBAD SYSLOG+WALL
NOTIFYFLAG SHUTDOWN SYSLOG+WALL
NOTIFYFLAG REPLBATT SYSLOG+WALL
NOTIFYFLAG NOCOMM SYSLOG+WALL

# Run scripts on events (optional)
# NOTIFYCMD /usr/local/bin/nut-notify.sh

# User to run as (leave default)
RUN_AS_USER root
```

### Configuration Options Explained

| Option | Description |
|--------|-------------|
| `MONITOR` | UPS to monitor and credentials |
| `MINSUPPLIES` | Minimum UPS units needed (usually 1) |
| `SHUTDOWNCMD` | Command to run for shutdown |
| `POLLFREQ` | How often to check UPS status |
| `POLLFREQALERT` | Poll frequency when on battery |
| `DEADTIME` | Seconds before declaring UPS unreachable |
| `FINALDELAY` | Wait time before final shutdown |

### Master vs Slave

- **Master**: The machine directly connected to the UPS. Can command UPS actions.
- **Slave**: Remote machines monitoring the UPS. Cannot command UPS, only monitor.

Clients are always configured as `slave`.

## Starting the Client

### Enable and Start Service

```bash
# Enable on boot
sudo systemctl enable nut-monitor

# Start the service
sudo systemctl start nut-monitor

# Check status
sudo systemctl status nut-monitor
```

### View Logs

```bash
# Real-time logs
journalctl -u nut-monitor -f

# Recent logs
journalctl -u nut-monitor --since "1 hour ago"
```

## Platform-Specific Setup

### Proxmox VE

```bash
# Install
apt update && apt install nut-client

# Configure
cat > /etc/nut/nut.conf << EOF
MODE=netclient
EOF

cat > /etc/nut/upsmon.conf << EOF
MONITOR myups@192.168.1.10 1 upsmon_remote your_password slave
MINSUPPLIES 1
SHUTDOWNCMD "/sbin/shutdown -h +0"
POLLFREQ 5
POLLFREQALERT 5
DEADTIME 15
FINALDELAY 5
EOF

# Start
systemctl enable nut-monitor
systemctl start nut-monitor
```

### TrueNAS SCALE

1. Go to **System Settings** → **Services**
2. Enable **UPS** service
3. Configure:
   - Mode: `Slave`
   - Remote Host: `192.168.1.10`
   - Remote Port: `3493`
   - UPS Name: `myups`
   - Username: `upsmon_remote`
   - Password: Your password

### Docker Container

Create a `docker-compose.yml`:

```yaml
version: '3.8'

services:
  nut-client:
    image: instantlinux/nut-upsd:latest
    container_name: nut-client
    environment:
      - MODE=netclient
      - MONITOR_HOST=192.168.1.10
      - MONITOR_USER=upsmon_remote
      - MONITOR_PASSWORD=your_password
      - UPS_NAME=myups
    network_mode: host
    restart: unless-stopped
```

Or use environment variables with a minimal image:

```yaml
version: '3.8'

services:
  nut-client:
    image: upshift/nut-upsd
    container_name: nut-client
    volumes:
      - ./nut-client/upsmon.conf:/etc/nut/upsmon.conf:ro
      - ./nut-client/nut.conf:/etc/nut/nut.conf:ro
    network_mode: host
    restart: unless-stopped
```

### Home Assistant

Add to `configuration.yaml`:

```yaml
sensor:
  - platform: nut
    host: 192.168.1.10
    port: 3493
    username: upsmon_remote
    password: your_password
    resources:
      - ups.status
      - ups.load
      - battery.charge
      - battery.runtime
      - input.voltage
      - output.voltage
```

### ESXi (via VM)

Deploy a lightweight Linux VM (Alpine, Ubuntu Server) configured as NUT client to trigger ESXi shutdown via SSH or API.

## Testing

### Verify Connection

```bash
# Check if you can reach the server
upsc myups@192.168.1.10

# Get specific values
upsc myups@192.168.1.10 battery.charge
upsc myups@192.168.1.10 ups.status
```

### Expected Output

```
battery.charge: 100
battery.runtime: 3600
device.type: ups
input.voltage: 120.0
output.voltage: 120.0
ups.load: 25
ups.status: OL
```

### Test Shutdown Sequence

**Warning:** This will actually shut down your machine!

```bash
# Simulate a forced shutdown (test only on non-production)
sudo upsmon -c fsd
```

### Check Service Status

```bash
# Service status
systemctl status nut-monitor

# Process check
ps aux | grep upsmon
```

## Notifications

### Email Notifications

Create `/usr/local/bin/nut-notify.sh`:

```bash
#!/bin/bash

# NUT notification script
NOTIFYTYPE=$1
UPSNAME=$2
MESSAGE="UPS Event: $NOTIFYTYPE on $UPSNAME at $(date)"

# Send email
echo "$MESSAGE" | mail -s "UPS Alert: $NOTIFYTYPE" admin@example.com

# Log to syslog
logger -t nut-notify "$MESSAGE"
```

Make it executable:

```bash
sudo chmod +x /usr/local/bin/nut-notify.sh
```

Add to `upsmon.conf`:

```ini
NOTIFYCMD /usr/local/bin/nut-notify.sh
```

### Slack/Discord Notifications

```bash
#!/bin/bash

WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
NOTIFYTYPE=$1
UPSNAME=$2

curl -X POST -H 'Content-type: application/json' \
  --data "{\"text\":\"🔋 UPS Alert: $NOTIFYTYPE on $UPSNAME\"}" \
  "$WEBHOOK_URL"
```

### Using upssched for Timed Events

Edit `/etc/nut/upssched.conf`:

```ini
CMDSCRIPT /usr/local/bin/upssched-cmd
PIPEFN /run/nut/upssched.pipe
LOCKFN /run/nut/upssched.lock

# Wait 30 seconds on battery before alerting
AT ONBATT * START-TIMER onbatt 30
AT ONLINE * CANCEL-TIMER onbatt
AT ONBATT * EXECUTE onbatt-immediate

# Low battery - immediate action
AT LOWBATT * EXECUTE lowbatt
```

Create `/usr/local/bin/upssched-cmd`:

```bash
#!/bin/bash

case $1 in
    onbatt)
        logger -t upssched "UPS on battery for 30+ seconds"
        # Send notification
        ;;
    lowbatt)
        logger -t upssched "UPS battery low - shutdown imminent"
        ;;
    onbatt-immediate)
        logger -t upssched "UPS switched to battery"
        ;;
esac
```

## Monitoring Multiple UPS

You can monitor multiple UPS units:

```ini
MONITOR ups1@server1 1 user password slave
MONITOR ups2@server2 1 user password slave
MINSUPPLIES 2
```

The client will only shut down when both UPS units are unavailable or on low battery.

## Troubleshooting

### "Connection refused"

```bash
# Check network connectivity
nc -zv 192.168.1.10 3493

# Check firewall on server
sudo ufw status  # Ubuntu
sudo firewall-cmd --list-ports  # RHEL
```

### "Access denied"

- Verify username and password in `upsmon.conf`
- Check `upsd.users` on the server has the correct user
- Ensure user has `upsmon slave` permission

### "UPS not found"

```bash
# List UPS on server
upsc -l 192.168.1.10

# Check UPS name matches exactly
```

### Connection Intermittent

```bash
# Increase deadtime
DEADTIME 25

# Check network stability
ping -c 100 192.168.1.10
```

### Debug Mode

```bash
# Stop service
sudo systemctl stop nut-monitor

# Run in debug mode
sudo upsmon -D
```

### Check Configuration

```bash
# Validate configuration
cat /etc/nut/upsmon.conf

# Check permissions
ls -la /etc/nut/
```

## Security Considerations

1. **Use strong passwords** in `upsd.users` and `upsmon.conf`
2. **Restrict network access** - use firewall rules to limit port 3493
3. **Use VPN** for remote monitoring across untrusted networks
4. **File permissions** - ensure config files are readable only by root/nut group

```bash
sudo chown root:nut /etc/nut/upsmon.conf
sudo chmod 640 /etc/nut/upsmon.conf
```

## Next Steps

- [Learn NUT CLI commands](cli-reference.md) for monitoring and testing
- Set up monitoring dashboards (Grafana, Home Assistant)
- Configure automated notifications
