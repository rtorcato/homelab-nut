# Smart Device Shutdown Guide

How to gracefully shut down UniFi, LG TVs, and other smart devices during power outages using NUT.

## Table of Contents

- [Overview](#overview)
- [Shutdown Options Comparison](#shutdown-options-comparison)
- [Home Assistant (Recommended)](#home-assistant-recommended)
- [UniFi Device Shutdown](#unifi-device-shutdown)
- [LG TV Shutdown](#lg-tv-shutdown)
- [Other Devices](#other-devices)
- [Direct Script Approach](#direct-script-approach)

## Overview

When UPS battery runs low, you may want to gracefully shut down:
- **UniFi** switches, access points, gateways
- **LG TVs** (WebOS devices)
- **Smart plugs** controlling non-smart devices
- **NAS devices** (Synology, QNAP)
- **IoT devices** that benefit from clean shutdown

## Shutdown Options Comparison

| Method | Pros | Cons |
|--------|------|------|
| **Home Assistant** | Central control, visual dashboard, many integrations | Requires HA setup |
| **Direct Scripts** | No dependencies, simple | Must maintain scripts, less flexible |
| **Node-RED** | Visual workflow, powerful | Additional service to run |

**Recommendation**: Use **Home Assistant** if you already have it or plan to. It provides the most flexibility and integrates easily with NUT.

## Home Assistant (Recommended)

Home Assistant is ideal because:
- Native NUT integration
- Supports 1000+ device types
- Visual automations
- Mobile notifications built-in
- Survives NUT restarts

### Install NUT Integration

1. Go to **Settings** → **Devices & Services** → **Add Integration**
2. Search for "Network UPS Tools (NUT)"
3. Configure:
   - Host: Your NUT server IP
   - Port: 3493
   - Username: `upsmon_remote`
   - Password: Your password
   - UPS Name: `myups`

### Available Sensors

After setup, you'll have sensors like:
- `sensor.myups_battery_charge`
- `sensor.myups_ups_status`
- `sensor.myups_ups_load`
- `sensor.myups_battery_runtime`
- `sensor.myups_input_voltage`

### Power Outage Automation

```yaml
# automations.yaml

# Notify on power outage
- alias: "UPS Power Outage - Notify"
  trigger:
    - platform: state
      entity_id: sensor.myups_ups_status
      to: "OB"  # On Battery
  action:
    - service: notify.mobile_app_your_phone
      data:
        title: "⚡ Power Outage"
        message: "UPS on battery. Charge: {{ states('sensor.myups_battery_charge') }}%"

# Shutdown devices when battery low
- alias: "UPS Low Battery - Shutdown Devices"
  trigger:
    - platform: numeric_state
      entity_id: sensor.myups_battery_charge
      below: 30
  condition:
    - condition: state
      entity_id: sensor.myups_ups_status
      state: "OB"
  action:
    # Turn off TVs
    - service: media_player.turn_off
      target:
        entity_id:
          - media_player.living_room_tv
          - media_player.bedroom_tv
    # Turn off smart plugs
    - service: switch.turn_off
      target:
        entity_id:
          - switch.office_equipment
          - switch.gaming_setup
    # Notify
    - service: notify.mobile_app_your_phone
      data:
        title: "🔋 Low Battery"
        message: "Shutting down non-essential devices"

# Critical battery - shutdown everything
- alias: "UPS Critical - Emergency Shutdown"
  trigger:
    - platform: numeric_state
      entity_id: sensor.myups_battery_charge
      below: 15
  condition:
    - condition: state
      entity_id: sensor.myups_ups_status
      state: "OB"
  action:
    - service: script.emergency_shutdown
```

### Emergency Shutdown Script

```yaml
# scripts.yaml

emergency_shutdown:
  alias: "Emergency UPS Shutdown"
  sequence:
    # Notify first
    - service: notify.all_devices
      data:
        title: "🚨 Emergency Shutdown"
        message: "Critical battery level - shutting down all devices"
    
    # Shutdown UniFi
    - service: shell_command.shutdown_unifi
    
    # Turn off all media players
    - service: media_player.turn_off
      target:
        entity_id: all
    
    # Turn off smart plugs (non-essential)
    - service: switch.turn_off
      target:
        entity_id:
          - switch.office_equipment
          - switch.entertainment_center
    
    # Delay for NAS to sync
    - delay:
        seconds: 30
    
    # Shutdown NAS
    - service: shell_command.shutdown_nas
```

## UniFi Device Shutdown

### Option 1: Home Assistant UniFi Integration

Install the [UniFi Network integration](https://www.home-assistant.io/integrations/unifi/):

```yaml
# Shell commands for UniFi (via SSH)
shell_command:
  shutdown_unifi_switch: >
    ssh -o StrictHostKeyChecking=no admin@192.168.1.2 'poweroff'
  
  shutdown_unifi_ap: >
    ssh -o StrictHostKeyChecking=no admin@192.168.1.3 'poweroff'
```

### Option 2: UniFi Controller API

Create a script to use UniFi Controller API:

```bash
#!/bin/bash
# /usr/local/bin/unifi-shutdown.sh

CONTROLLER="https://192.168.1.1:8443"
USERNAME="admin"
PASSWORD="your_password"
SITE="default"

# Login and get cookie
COOKIE=$(mktemp)
curl -s -k -X POST "$CONTROLLER/api/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
  -c "$COOKIE"

# Get device list
DEVICES=$(curl -s -k -X GET "$CONTROLLER/api/s/$SITE/stat/device" \
  -b "$COOKIE" | jq -r '.data[].mac')

# Note: UniFi doesn't have a clean "shutdown" API command
# Best approach is to disable PoE ports or use SSH

# Logout
curl -s -k -X POST "$CONTROLLER/api/logout" -b "$COOKIE"
rm "$COOKIE"
```

### Option 3: Disable PoE (Switches)

More graceful than hard shutdown - disable PoE ports:

```yaml
# Home Assistant automation
- alias: "UPS Low - Disable Non-Essential PoE"
  trigger:
    - platform: numeric_state
      entity_id: sensor.myups_battery_charge
      below: 40
  action:
    # Uses UniFi integration to disable PoE on specific ports
    - service: unifi.reconnect_client
      data:
        # Or use shell command to SSH and disable PoE
```

### Option 4: SSH Direct Shutdown

```bash
#!/bin/bash
# Shutdown UniFi devices via SSH

# USG/UDM
ssh admin@192.168.1.1 'poweroff'

# Switches
ssh admin@192.168.1.2 'poweroff'

# Access Points (will reboot when power returns)
ssh admin@192.168.1.3 'poweroff'
```

Setup SSH key authentication:
```bash
ssh-keygen -t ed25519
ssh-copy-id admin@192.168.1.1
```

## LG TV Shutdown

### Option 1: Home Assistant LG WebOS Integration

The [LG WebOS integration](https://www.home-assistant.io/integrations/webostv/) provides full control:

```yaml
# Turn off TV during power outage
- alias: "UPS on Battery - Turn Off TVs"
  trigger:
    - platform: state
      entity_id: sensor.myups_ups_status
      to: "OB"
  action:
    - delay:
        minutes: 2  # Wait in case brief outage
    - condition: state
      entity_id: sensor.myups_ups_status
      state: "OB"
    - service: media_player.turn_off
      target:
        entity_id: media_player.lg_tv_living_room
```

### Option 2: Wake-on-LAN / LG Control Script

```bash
#!/bin/bash
# Control LG TV via command line

TV_IP="192.168.1.50"
TV_MAC="AA:BB:CC:DD:EE:FF"

# Install lgtv package
# pip install lgtv2

# Turn off
lgtv --ssl --name MyTV --host $TV_IP command system.turnOff

# Or use webOS API directly
curl -X POST "http://$TV_IP:3000/service/system/turnOff"
```

### Option 3: HDMI-CEC

If TV is connected to a device with CEC control:

```bash
# Using cec-client
echo 'standby 0' | cec-client -s -d 1
```

## Other Devices

### Synology NAS

```yaml
# Home Assistant shell command
shell_command:
  shutdown_synology: >
    ssh admin@192.168.1.100 'sudo shutdown -h now'
```

Or use Synology's API:
```bash
curl "http://nas:5000/webapi/entry.cgi?api=SYNO.Core.System&method=shutdown&version=1&_sid=$SESSION_ID"
```

### QNAP NAS

```bash
ssh admin@qnap 'poweroff'
```

### Smart Plugs (for dumb devices)

Control via Home Assistant or directly:

```yaml
# Turn off smart plugs for UPS-protected but non-essential devices
- service: switch.turn_off
  target:
    entity_id:
      - switch.monitor_plug
      - switch.speakers_plug
      - switch.gaming_pc_plug
```

### ESXi / Proxmox VMs

```yaml
# Proxmox shutdown via API
shell_command:
  shutdown_proxmox_vms: >
    curl -k -X POST 'https://proxmox:8006/api2/json/nodes/pve/qemu/100/status/shutdown'
    -H 'Authorization: PVEAPIToken=user@pam!token=TOKEN_VALUE'
```

## Direct Script Approach

If not using Home Assistant, create a shutdown orchestration script:

### Master Shutdown Script

Create `/usr/local/bin/ups-shutdown-all.sh`:

```bash
#!/bin/bash
#
# UPS Emergency Shutdown Script
# Called when battery is critically low
#

LOG="/var/log/ups-shutdown.log"
exec >> "$LOG" 2>&1

echo "=========================================="
echo "UPS Shutdown initiated at $(date)"
echo "=========================================="

# Configuration
SYNOLOGY_IP="192.168.1.100"
QNAP_IP="192.168.1.101"
LG_TV_IP="192.168.1.50"
UNIFI_SWITCH_IP="192.168.1.2"

# Function to shutdown with timeout
shutdown_device() {
    local name=$1
    local cmd=$2
    echo "Shutting down $name..."
    timeout 30 bash -c "$cmd" && echo "  ✓ $name shutdown" || echo "  ✗ $name failed"
}

# 1. Turn off TVs (fast, no data loss concern)
echo "--- Phase 1: Media devices ---"
shutdown_device "LG TV" "lgtv --host $LG_TV_IP command system.turnOff 2>/dev/null"

# 2. Stop non-essential services
echo "--- Phase 2: Services ---"
# Add any Docker containers, VMs, etc.

# 3. Shutdown NAS devices (need time to sync)
echo "--- Phase 3: Storage devices ---"
shutdown_device "Synology" "ssh -o ConnectTimeout=10 admin@$SYNOLOGY_IP 'sudo shutdown -h now'"
shutdown_device "QNAP" "ssh -o ConnectTimeout=10 admin@$QNAP_IP 'poweroff'"

# 4. Wait for storage to sync
echo "Waiting 60s for storage sync..."
sleep 60

# 5. Network devices (last, as they may affect connectivity)
echo "--- Phase 4: Network devices ---"
shutdown_device "UniFi Switch" "ssh -o ConnectTimeout=10 admin@$UNIFI_SWITCH_IP 'poweroff'"

echo "=========================================="
echo "Shutdown sequence completed at $(date)"
echo "=========================================="
```

### Integrate with NUT

Add to `/etc/nut/upsmon.conf`:

```ini
# Run shutdown script on low battery
NOTIFYCMD /usr/local/bin/ups-notify-handler.sh
NOTIFYFLAG LOWBATT EXEC
```

Create `/usr/local/bin/ups-notify-handler.sh`:

```bash
#!/bin/bash

NOTIFY_TYPE="$1"

case "$NOTIFY_TYPE" in
    LOWBATT)
        /usr/local/bin/ups-shutdown-all.sh &
        ;;
esac
```

### Using upssched for Staged Shutdown

```ini
# /etc/nut/upssched.conf

CMDSCRIPT /usr/local/bin/upssched-cmd
PIPEFN /run/nut/upssched.pipe
LOCKFN /run/nut/upssched.lock

# When on battery, start timers
AT ONBATT * START-TIMER stage1 60      # Non-essential after 1 min
AT ONBATT * START-TIMER stage2 180     # NAS after 3 min
AT ONLINE * CANCEL-TIMER stage1
AT ONLINE * CANCEL-TIMER stage2

# Low battery = immediate action
AT LOWBATT * EXECUTE emergency
```

```bash
#!/bin/bash
# /usr/local/bin/upssched-cmd

case "$1" in
    stage1)
        # Turn off TVs, non-essential
        logger "UPS: Stage 1 shutdown - non-essential devices"
        /usr/local/bin/shutdown-nonessential.sh
        ;;
    stage2)
        # Shutdown NAS
        logger "UPS: Stage 2 shutdown - storage devices"
        /usr/local/bin/shutdown-storage.sh
        ;;
    emergency)
        # Everything
        logger "UPS: Emergency shutdown - all devices"
        /usr/local/bin/ups-shutdown-all.sh
        ;;
esac
```

## Summary: Recommended Setup

1. **Home Assistant + NUT integration** for centralized control
2. **Staged shutdown** based on battery level:
   - 50%: Notify only
   - 40%: Turn off TVs, non-essential
   - 30%: Shutdown NAS devices (need sync time)
   - 20%: Shutdown network gear
   - 15%: Final host shutdowns
3. **Test regularly** - simulate power outages to ensure everything works
4. **Monitor logs** - check that shutdown commands execute properly
