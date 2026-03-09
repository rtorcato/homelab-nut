# NUT Server Setup Guide

This guide covers installing and configuring NUT server on the machine directly connected to your UPS.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration Files](#configuration-files)
- [Configuring the UPS Driver](#configuring-the-ups-driver)
- [Configuring the NUT Server](#configuring-the-nut-server)
- [Creating Users](#creating-users)
- [Setting Up upsmon](#setting-up-upsmon)
- [Starting Services](#starting-services)
- [Firewall Configuration](#firewall-configuration)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- Linux-based system (Debian/Ubuntu, RHEL/CentOS, etc.)
- UPS connected via USB or serial port
- Root/sudo access

## Installation

### Debian/Ubuntu

```bash
sudo apt update
sudo apt install nut nut-server nut-client
```

### RHEL/CentOS/Fedora

```bash
sudo dnf install nut nut-client
```

### Proxmox VE

```bash
apt update
apt install nut nut-server nut-client
```

## Configuration Files

NUT configuration files are located in `/etc/nut/`:

| File | Purpose |
|------|---------|
| `nut.conf` | Sets NUT mode (standalone, netserver, netclient) |
| `ups.conf` | Defines UPS devices and their drivers |
| `upsd.conf` | NUT server daemon configuration |
| `upsd.users` | User authentication for NUT daemon |
| `upsmon.conf` | UPS monitor configuration |

## Identify Your UPS

Before configuring, identify your UPS:

```bash
# List USB devices
lsusb

# Example output:
# Bus 001 Device 003: ID 051d:0002 American Power Conversion Uninterruptible Power Supply
```

Use the [NUT Hardware Compatibility List](https://networkupstools.org/stable-hcl.html) to find the correct driver for your UPS.

## Configuring NUT Mode

Edit `/etc/nut/nut.conf`:

```bash
sudo nano /etc/nut/nut.conf
```

Set the mode based on your setup:

```ini
# standalone: UPS connected, no network sharing
# netserver: UPS connected, share status over network (most common for homelab)
# netclient: No UPS connected, monitor remote NUT server

MODE=netserver
```

## Configuring the UPS Driver

Edit `/etc/nut/ups.conf`:

```bash
sudo nano /etc/nut/ups.conf
```

### Example for APC UPS (USB)

```ini
[myups]
    driver = usbhid-ups
    port = auto
    desc = "APC Back-UPS 1500"
    
    # Optional: Set polling interval (seconds)
    pollinterval = 5
```

### Example for CyberPower UPS (USB)

```ini
[cyberpower]
    driver = usbhid-ups
    port = auto
    desc = "CyberPower CP1500"
    vendorid = 0764
```

### Example for Serial Connection

```ini
[serialups]
    driver = apcsmart
    port = /dev/ttyS0
    cable = 940-0024C
    desc = "APC Smart-UPS 1000"
```

### Common Drivers

| Driver | Supported UPS |
|--------|---------------|
| `usbhid-ups` | Most USB UPS (APC, CyberPower, Eaton, etc.) |
| `apcsmart` | APC Smart-UPS (serial) |
| `blazer_usb` | Various Chinese UPS brands |
| `nutdrv_qx` | Various Q* protocol UPS |
| `snmp-ups` | Network-managed UPS via SNMP |

## Configuring the NUT Server

Edit `/etc/nut/upsd.conf`:

```bash
sudo nano /etc/nut/upsd.conf
```

```ini
# Listen on all interfaces (for network clients)
LISTEN 0.0.0.0 3493

# Or listen on specific IP
# LISTEN 192.168.1.100 3493

# Also listen on localhost
LISTEN 127.0.0.1 3493

# Maximum age of data before marking stale (seconds)
MAXAGE 15
```

## Creating Users

Edit `/etc/nut/upsd.users`:

```bash
sudo nano /etc/nut/upsd.users
```

```ini
# Admin user (for local server)
[admin]
    password = your_secure_admin_password
    actions = SET
    instcmds = ALL

# Monitor user for local upsmon
[upsmon_local]
    password = your_secure_local_password
    upsmon master

# Monitor user for remote clients
[upsmon_remote]
    password = your_secure_remote_password
    upsmon slave
```

**Important:** Choose strong passwords and keep them secure!

## Setting Up upsmon

The `upsmon` daemon monitors UPS status and triggers shutdowns.

Edit `/etc/nut/upsmon.conf`:

```bash
sudo nano /etc/nut/upsmon.conf
```

```ini
# Monitor the local UPS
# Format: MONITOR <ups>@<host> <powervalue> <user> <password> <type>
# powervalue: number of power supplies (usually 1)
# type: master (can command UPS) or slave (monitor only)

MONITOR myups@localhost 1 upsmon_local your_secure_local_password master

# Minimum battery percentage before shutdown
MINSUPPLIES 1

# Shutdown command
SHUTDOWNCMD "/sbin/shutdown -h +0"

# Notification command (optional)
NOTIFYCMD /usr/sbin/upssched

# Polling frequency (seconds)
POLLFREQ 5

# Poll frequency when on battery
POLLFREQALERT 5

# Time to wait before shutting down (seconds)
HOSTSYNC 15

# Shutdown wait time
DEADTIME 15

# Battery low notification
NOTIFYFLAG ONBATT SYSLOG+WALL
NOTIFYFLAG LOWBATT SYSLOG+WALL
NOTIFYFLAG ONLINE SYSLOG+WALL
NOTIFYFLAG COMMOK SYSLOG+WALL
NOTIFYFLAG COMMBAD SYSLOG+WALL
NOTIFYFLAG SHUTDOWN SYSLOG+WALL
NOTIFYFLAG REPLBATT SYSLOG+WALL

# Power off UPS after host shutdown
POWERDOWNFLAG /etc/killpower
```

## Starting Services

### Enable and Start Services

```bash
# Start the UPS driver
sudo systemctl enable nut-driver
sudo systemctl start nut-driver

# Start the NUT server
sudo systemctl enable nut-server
sudo systemctl start nut-server

# Start the monitor
sudo systemctl enable nut-monitor
sudo systemctl start nut-monitor
```

### Check Service Status

```bash
sudo systemctl status nut-driver
sudo systemctl status nut-server
sudo systemctl status nut-monitor
```

### View Logs

```bash
# Journal logs
journalctl -u nut-server -f

# System logs
tail -f /var/log/syslog | grep -i ups
```

## Firewall Configuration

Open port 3493 for remote clients:

### UFW (Ubuntu)

```bash
sudo ufw allow 3493/tcp
sudo ufw reload
```

### firewalld (RHEL/CentOS)

```bash
sudo firewall-cmd --permanent --add-port=3493/tcp
sudo firewall-cmd --reload
```

### iptables

```bash
sudo iptables -A INPUT -p tcp --dport 3493 -j ACCEPT
```

## Testing

### Test UPS Driver

```bash
# Start driver manually for testing
sudo upsdrvctl start

# Check if driver is running
ps aux | grep ups
```

### Test Server Connection

```bash
# List available UPS devices
upsc -l

# Query UPS status
upsc myups@localhost

# Get specific variable
upsc myups@localhost battery.charge
```

### Test Remote Access

From a remote machine:

```bash
upsc myups@<server-ip>
```

## Troubleshooting

### Common Issues

#### "Can't connect to UPS"

```bash
# Check USB permissions
ls -la /dev/bus/usb/*/*

# Add user to nut group
sudo usermod -a -G nut $USER

# Create udev rule for UPS
echo 'SUBSYSTEM=="usb", ATTR{idVendor}=="051d", MODE="0664", GROUP="nut"' | \
sudo tee /etc/udev/rules.d/99-ups.rules
sudo udevadm control --reload-rules
```

#### "Driver not found"

```bash
# List available drivers
ls /lib/nut/

# Test driver manually
sudo /lib/nut/usbhid-ups -a myups -DDDDD
```

#### "Connection refused" from remote clients

1. Check firewall settings
2. Verify `LISTEN` directive in `upsd.conf`
3. Check if server is running: `sudo systemctl status nut-server`

#### Permission Issues

```bash
# Fix configuration file permissions
sudo chown root:nut /etc/nut/*.conf
sudo chmod 640 /etc/nut/*.conf
sudo chmod 640 /etc/nut/upsd.users
```

### Debug Mode

Run services in debug mode:

```bash
# Stop services first
sudo systemctl stop nut-server nut-driver nut-monitor

# Run driver in debug mode
sudo upsdrvctl -D start

# Run server in debug mode
sudo upsd -D

# Run monitor in debug mode
sudo upsmon -D
```

## Next Steps

- [Configure NUT Clients](client-setup.md) on other machines
- [Learn NUT CLI commands](cli-reference.md) for monitoring and management
