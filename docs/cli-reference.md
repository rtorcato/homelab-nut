# NUT CLI Reference

This guide covers the command-line tools provided by NUT for monitoring, controlling, and troubleshooting your UPS setup.

## Table of Contents

- [Quick Reference](#quick-reference)
- [upsc - UPS Client](#upsc---ups-client)
- [upscmd - UPS Commands](#upscmd---ups-commands)
- [upsrw - Read/Write Variables](#upsrw---readwrite-variables)
- [upsdrvctl - Driver Control](#upsdrvctl---driver-control)
- [upsmon - Monitor Control](#upsmon---monitor-control)
- [upsd - Server Daemon](#upsd---server-daemon)
- [upslog - Data Logging](#upslog---data-logging)
- [Useful Scripts](#useful-scripts)

## Quick Reference

| Command | Purpose |
|---------|---------|
| `upsc` | Query UPS status and variables |
| `upscmd` | Send commands to UPS |
| `upsrw` | Read/write UPS variables |
| `upsdrvctl` | Control UPS drivers |
| `upsmon` | Control UPS monitor |
| `upsd` | NUT server daemon |
| `upslog` | Log UPS data to file |

## upsc - UPS Client

The `upsc` command queries UPS status and variables. This is your primary tool for checking UPS health.

### Basic Usage

```bash
# List all UPS devices on a host
upsc -l
upsc -l localhost
upsc -l 192.168.1.10

# Get all variables for a UPS
upsc myups
upsc myups@localhost
upsc myups@192.168.1.10

# Get a specific variable
upsc myups@localhost battery.charge
upsc myups@localhost ups.status
```

### Common Variables

#### Battery Information

```bash
# Battery charge percentage (0-100)
upsc myups battery.charge

# Estimated runtime in seconds
upsc myups battery.runtime

# Battery voltage
upsc myups battery.voltage

# Battery status (charging, discharging, etc.)
upsc myups battery.charger.status
```

#### UPS Status

```bash
# UPS status flags
upsc myups ups.status

# Load percentage
upsc myups ups.load

# UPS model
upsc myups ups.model

# UPS manufacturer
upsc myups ups.mfr

# UPS serial number
upsc myups ups.serial

# UPS temperature (if available)
upsc myups ups.temperature
```

#### Power Information

```bash
# Input voltage
upsc myups input.voltage

# Output voltage
upsc myups output.voltage

# Input frequency
upsc myups input.frequency

# Output frequency
upsc myups output.frequency

# Real power (watts)
upsc myups ups.realpower

# Apparent power (VA)
upsc myups ups.power
```

### UPS Status Codes

The `ups.status` field returns status flags:

| Flag | Meaning |
|------|---------|
| `OL` | Online (on mains power) |
| `OB` | On Battery |
| `LB` | Low Battery |
| `HB` | High Battery |
| `RB` | Replace Battery |
| `CHRG` | Charging |
| `DISCHRG` | Discharging |
| `BYPASS` | On Bypass |
| `CAL` | Calibrating |
| `OFF` | UPS is off |
| `OVER` | Overloaded |
| `TRIM` | Trimming voltage |
| `BOOST` | Boosting voltage |
| `FSD` | Forced Shutdown |

Multiple flags can appear together, e.g., `OL CHRG` (online and charging).

### Output Formatting

```bash
# Show variable names only
upsc -l myups

# Show variable values only (for scripting)
upsc myups battery.charge 2>/dev/null

# Combine with grep
upsc myups | grep battery

# Format output with awk
upsc myups | awk -F': ' '/battery/ {print $1, "=", $2}'
```

## upscmd - UPS Commands

The `upscmd` command sends instant commands to the UPS (e.g., beep, test, shutdown).

### List Available Commands

```bash
# List commands supported by UPS
upscmd -l myups
upscmd -l myups@localhost

# Example output:
# Instant commands supported on UPS [myups]:
# beeper.disable - Disable the UPS beeper
# beeper.enable - Enable the UPS beeper
# beeper.mute - Mute the UPS beeper
# load.off - Turn off the load immediately
# load.on - Turn on the load immediately
# shutdown.return - Turn off the load and return when power is back
# shutdown.stayoff - Turn off the load and remain off
# test.battery.start - Start a battery test
# test.battery.stop - Stop the battery test
```

### Execute Commands

```bash
# Syntax: upscmd -u <user> -p <password> <ups> <command>
upscmd -u admin -p password myups beeper.disable

# Interactive mode (prompts for password)
upscmd -u admin myups test.battery.start
```

### Common Commands

```bash
# Beeper control
upscmd -u admin -p password myups beeper.disable
upscmd -u admin -p password myups beeper.enable
upscmd -u admin -p password myups beeper.mute

# Battery test
upscmd -u admin -p password myups test.battery.start
upscmd -u admin -p password myups test.battery.stop
upscmd -u admin -p password myups test.battery.start.quick
upscmd -u admin -p password myups test.battery.start.deep

# Load control (DANGEROUS!)
upscmd -u admin -p password myups load.off
upscmd -u admin -p password myups load.on

# Shutdown commands (DANGEROUS!)
upscmd -u admin -p password myups shutdown.return    # Off, returns when power restored
upscmd -u admin -p password myups shutdown.stayoff   # Off, stays off

# Calibration
upscmd -u admin -p password myups calibrate.start
upscmd -u admin -p password myups calibrate.stop
```

### Sending Commands to Remote UPS

```bash
upscmd -u admin -p password myups@192.168.1.10 beeper.disable
```

## upsrw - Read/Write Variables

The `upsrw` command reads and writes UPS configuration variables.

### List Writable Variables

```bash
# Show all writable variables
upsrw myups
upsrw myups@localhost

# Example output:
# [battery.charge.low]
# Current value: 20
# Description: Remaining battery level when UPS switches to LB
# Type: STRING
# Maximum length: 10
```

### Read Variables

```bash
upsrw myups@localhost | grep -A3 "battery.charge.low"
```

### Write Variables

```bash
# Syntax: upsrw -s <variable>=<value> -u <user> -p <password> <ups>

# Set low battery threshold
upsrw -s battery.charge.low=30 -u admin -p password myups

# Set low battery runtime threshold (seconds)
upsrw -s battery.runtime.low=300 -u admin -p password myups

# Set UPS name
upsrw -s ups.id="Server Room UPS" -u admin -p password myups
```

### Common Writable Variables

| Variable | Description |
|----------|-------------|
| `battery.charge.low` | Low battery threshold (%) |
| `battery.runtime.low` | Low runtime threshold (seconds) |
| `ups.delay.shutdown` | Delay before shutdown (seconds) |
| `ups.delay.start` | Delay before restart (seconds) |
| `ups.id` | UPS identifier string |
| `outlet.n.delay.shutdown` | Outlet shutdown delay |
| `outlet.n.delay.start` | Outlet startup delay |

## upsdrvctl - Driver Control

The `upsdrvctl` command manages UPS drivers.

### Start/Stop Drivers

```bash
# Start all configured drivers
sudo upsdrvctl start

# Start specific UPS driver
sudo upsdrvctl start myups

# Stop all drivers
sudo upsdrvctl stop

# Stop specific driver
sudo upsdrvctl stop myups

# Reload driver configuration
sudo upsdrvctl reload
```

### Debug Mode

```bash
# Start with debug output (more D's = more verbose)
sudo upsdrvctl -D start
sudo upsdrvctl -DD start
sudo upsdrvctl -DDD start

# Test specific driver without daemonizing
sudo /lib/nut/usbhid-ups -a myups -DDDDD
```

### Options

```bash
# Specify alternate config directory
sudo upsdrvctl -c /etc/nut start

# Maximum startup timeout (seconds)
sudo upsdrvctl -t 30 start
```

## upsmon - Monitor Control

The `upsmon` daemon monitors UPS status and triggers actions.

### Control Commands

```bash
# Force shutdown (FSD) - DANGEROUS!
sudo upsmon -c fsd

# Reload configuration
sudo upsmon -c reload

# Stop the monitor
sudo upsmon -c stop
```

### Debug Mode

```bash
# Run in foreground with debug
sudo upsmon -D

# More verbose
sudo upsmon -DD

# Don't fork, output to stdout
sudo upsmon -D -f
```

### Status Check

```bash
# Check if upsmon is running
ps aux | grep upsmon
pgrep upsmon
```

## upsd - Server Daemon

The `upsd` daemon serves UPS data to clients.

### Control

```bash
# Start server
sudo upsd

# Start in debug mode
sudo upsd -D

# Reload configuration
sudo upsd -c reload

# Stop server
sudo upsd -c stop

# Display connected clients
sudo upsd -c status
```

### Connection Info

```bash
# Show version
upsd -V

# List defined UPS devices
upsc -l localhost
```

## upslog - Data Logging

The `upslog` command logs UPS data to a file for monitoring/analysis.

### Basic Usage

```bash
# Log to file every 30 seconds
upslog -s myups@localhost -l /var/log/ups.log -i 30

# Log specific variables
upslog -s myups@localhost -l /var/log/ups.log -i 60 \
  -f "TIME: %TIME% Battery: %VAR battery.charge% Status: %VAR ups.status%"
```

### Format Specifiers

| Specifier | Description |
|-----------|-------------|
| `%TIME%` | Current timestamp |
| `%ETIME%` | Epoch time |
| `%HOST%` | UPS hostname |
| `%UPSHOST%` | Full UPS@host |
| `%VAR varname%` | Variable value |

### Example Log Format

```bash
upslog -s myups@localhost -l /var/log/ups.log -i 60 \
  -f "%TIME% %UPSHOST% %VAR battery.charge%% %VAR ups.load%% %VAR ups.status%"

# Output:
# 2024-01-15 10:30:00 myups@localhost 100 25 OL
# 2024-01-15 10:31:00 myups@localhost 100 23 OL
```

### Run as Daemon

```bash
# Background process
nohup upslog -s myups@localhost -l /var/log/ups.log -i 60 &
```

## Useful Scripts

### Status Dashboard Script

Create `/usr/local/bin/ups-status`:

```bash
#!/bin/bash

UPS="${1:-myups@localhost}"

echo "========================================"
echo "        UPS Status: $UPS"
echo "========================================"
echo ""

# Status
STATUS=$(upsc $UPS ups.status 2>/dev/null)
echo "Status:          $STATUS"

# Battery
CHARGE=$(upsc $UPS battery.charge 2>/dev/null)
RUNTIME=$(upsc $UPS battery.runtime 2>/dev/null)
RUNTIME_MIN=$((RUNTIME / 60))
echo "Battery:         ${CHARGE}%"
echo "Runtime:         ${RUNTIME_MIN} minutes"

# Load
LOAD=$(upsc $UPS ups.load 2>/dev/null)
echo "Load:            ${LOAD}%"

# Power
INPUT_V=$(upsc $UPS input.voltage 2>/dev/null)
OUTPUT_V=$(upsc $UPS output.voltage 2>/dev/null)
echo "Input Voltage:   ${INPUT_V}V"
echo "Output Voltage:  ${OUTPUT_V}V"

echo ""
echo "========================================"
```

### Battery Monitor Script

Create `/usr/local/bin/ups-monitor`:

```bash
#!/bin/bash

UPS="${1:-myups@localhost}"
LOW_THRESHOLD=30
CRIT_THRESHOLD=10

while true; do
    CHARGE=$(upsc $UPS battery.charge 2>/dev/null)
    STATUS=$(upsc $UPS ups.status 2>/dev/null)
    
    if [[ "$STATUS" == *"OB"* ]]; then
        echo "$(date): ON BATTERY - Charge: ${CHARGE}%"
        
        if [[ $CHARGE -lt $CRIT_THRESHOLD ]]; then
            echo "CRITICAL: Battery below ${CRIT_THRESHOLD}%!"
            # Add notification command here
        elif [[ $CHARGE -lt $LOW_THRESHOLD ]]; then
            echo "WARNING: Battery below ${LOW_THRESHOLD}%"
        fi
    fi
    
    sleep 60
done
```

### JSON Output Script

Create `/usr/local/bin/ups-json`:

```bash
#!/bin/bash

UPS="${1:-myups@localhost}"

echo "{"
echo "  \"ups\": \"$UPS\","
echo "  \"timestamp\": \"$(date -Iseconds)\","
echo "  \"status\": \"$(upsc $UPS ups.status 2>/dev/null)\","
echo "  \"battery\": {"
echo "    \"charge\": $(upsc $UPS battery.charge 2>/dev/null || echo null),"
echo "    \"runtime\": $(upsc $UPS battery.runtime 2>/dev/null || echo null),"
echo "    \"voltage\": $(upsc $UPS battery.voltage 2>/dev/null || echo null)"
echo "  },"
echo "  \"load\": $(upsc $UPS ups.load 2>/dev/null || echo null),"
echo "  \"input_voltage\": $(upsc $UPS input.voltage 2>/dev/null || echo null),"
echo "  \"output_voltage\": $(upsc $UPS output.voltage 2>/dev/null || echo null)"
echo "}"
```

### All Variables Export

```bash
# Export all UPS variables as environment variables
eval $(upsc myups@localhost | sed 's/: /="/;s/$/"/' | sed 's/\./_/g')
echo "Battery charge: $battery_charge"
```

### Watch Mode

```bash
# Continuous monitoring with watch
watch -n 5 'upsc myups@localhost | grep -E "battery|ups.status|ups.load"'

# Or with a custom script
watch -n 5 /usr/local/bin/ups-status
```

## Troubleshooting with CLI

### Check Driver Communication

```bash
# List available drivers
ls /lib/nut/

# Test driver directly
sudo /lib/nut/usbhid-ups -a myups -DDDDD

# Check USB connection
lsusb | grep -i ups
```

### Debug Server

```bash
# Check server config
cat /etc/nut/upsd.conf

# Test server binding
netstat -tlnp | grep 3493
ss -tlnp | grep 3493

# Run server in debug mode
sudo upsd -D
```

### Debug Client Connection

```bash
# Test raw connection
nc -v 192.168.1.10 3493

# Then type:
# LIST UPS
# LIST VAR myups

# Test with timeout
timeout 5 upsc myups@192.168.1.10
```

### Common Issues

```bash
# Permission denied
sudo chmod 660 /dev/bus/usb/*/*
sudo chown root:nut /dev/bus/usb/*/*

# Check NUT user exists
id nut

# Verify config syntax (look for errors)
sudo upsdrvctl start 2>&1 | head -20
```

## Quick Command Cheat Sheet

```bash
# Check UPS status
upsc myups

# Get battery percentage
upsc myups battery.charge

# Check if on battery
upsc myups ups.status | grep -q "OB" && echo "On Battery" || echo "On Mains"

# Silence beeper
upscmd -u admin -p pass myups beeper.disable

# Run battery test
upscmd -u admin -p pass myups test.battery.start.quick

# Restart drivers
sudo systemctl restart nut-driver

# Check all NUT services
systemctl status nut-server nut-driver nut-monitor

# View live logs
journalctl -u nut-server -u nut-driver -u nut-monitor -f
```
