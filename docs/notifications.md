# NUT Notifications & Alerts

Configure notifications for UPS events via Slack, Discord, email, Pushover, and more.

## Table of Contents

- [Overview](#overview)
- [Notification Architecture](#notification-architecture)
- [Slack Notifications](#slack-notifications)
- [Discord Notifications](#discord-notifications)
- [Pushover Notifications](#pushover-notifications)
- [Email Notifications](#email-notifications)
- [Telegram Notifications](#telegram-notifications)
- [Ntfy Notifications](#ntfy-notifications)
- [Home Assistant Integration](#home-assistant-integration)
- [Using upssched for Timed Alerts](#using-upssched-for-timed-alerts)

## Overview

NUT can trigger notifications via:
1. **NOTIFYCMD** - Run a script on specific events
2. **upssched** - Schedule delayed notifications (avoid alert spam)
3. **External monitoring** - Prometheus alerts, Home Assistant automations

## Notification Architecture

```
UPS Event → upsmon → NOTIFYCMD → notification script → Slack/Discord/etc.
                  ↘
                   upssched → delayed actions → script
```

## Basic Setup

### Step 1: Create Notification Script

Create `/usr/local/bin/nut-notify.sh`:

```bash
#!/bin/bash
#
# NUT Notification Script
# Called by upsmon on UPS events
#
# Arguments: $1 = NOTIFYTYPE, $2 = UPSNAME, $3 = MESSAGE (optional)
#

NOTIFYTYPE="$1"
UPSNAME="$2"
MESSAGE="${3:-UPS event: $NOTIFYTYPE}"

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
HOSTNAME=$(hostname)

# Log the event
logger -t nut-notify "[$NOTIFYTYPE] $UPSNAME: $MESSAGE"

# Source your notification config
CONFIG_FILE="/etc/nut/notify.conf"
[ -f "$CONFIG_FILE" ] && source "$CONFIG_FILE"

# Call notification functions based on what's configured
[ -n "$SLACK_WEBHOOK" ] && send_slack
[ -n "$DISCORD_WEBHOOK" ] && send_discord
[ -n "$PUSHOVER_TOKEN" ] && send_pushover
[ -n "$TELEGRAM_BOT_TOKEN" ] && send_telegram
[ -n "$NTFY_TOPIC" ] && send_ntfy
[ -n "$EMAIL_TO" ] && send_email

exit 0
```

Make it executable:
```bash
chmod +x /usr/local/bin/nut-notify.sh
```

### Step 2: Configure upsmon

Add to `/etc/nut/upsmon.conf`:

```ini
# Enable notifications
NOTIFYCMD /usr/local/bin/nut-notify.sh

# Configure which events trigger notifications
NOTIFYFLAG ONLINE    SYSLOG+WALL+EXEC
NOTIFYFLAG ONBATT    SYSLOG+WALL+EXEC
NOTIFYFLAG LOWBATT   SYSLOG+WALL+EXEC
NOTIFYFLAG FSD       SYSLOG+WALL+EXEC
NOTIFYFLAG COMMOK    SYSLOG+EXEC
NOTIFYFLAG COMMBAD   SYSLOG+WALL+EXEC
NOTIFYFLAG SHUTDOWN  SYSLOG+WALL+EXEC
NOTIFYFLAG REPLBATT  SYSLOG+WALL+EXEC
NOTIFYFLAG NOCOMM    SYSLOG+WALL+EXEC

# Custom notification messages
NOTIFYMSG ONLINE     "UPS %s is online (power restored)"
NOTIFYMSG ONBATT     "UPS %s is on battery!"
NOTIFYMSG LOWBATT    "UPS %s battery is LOW - shutdown imminent!"
NOTIFYMSG FSD        "UPS %s: Forced shutdown in progress"
NOTIFYMSG COMMOK     "UPS %s: Communication restored"
NOTIFYMSG COMMBAD    "UPS %s: Communication lost!"
NOTIFYMSG SHUTDOWN   "System shutdown initiated by UPS %s"
NOTIFYMSG REPLBATT   "UPS %s: Battery needs replacement"
NOTIFYMSG NOCOMM     "UPS %s: Not responding"
```

Restart upsmon:
```bash
sudo systemctl restart nut-monitor
```

## Slack Notifications

### Create Slack Webhook

1. Go to https://api.slack.com/apps
2. Create New App → From scratch
3. Select your workspace
4. Go to **Incoming Webhooks** → Activate
5. Click **Add New Webhook to Workspace**
6. Select channel and copy webhook URL

### Slack Notification Script

Create `/etc/nut/notify.conf`:

```bash
# Slack Configuration
SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
SLACK_CHANNEL="#alerts"
```

Add to `/usr/local/bin/nut-notify.sh`:

```bash
send_slack() {
    # Set emoji and color based on event type
    case "$NOTIFYTYPE" in
        ONLINE)
            EMOJI=":white_check_mark:"
            COLOR="good"
            ;;
        ONBATT)
            EMOJI=":warning:"
            COLOR="warning"
            ;;
        LOWBATT|FSD|SHUTDOWN)
            EMOJI=":rotating_light:"
            COLOR="danger"
            ;;
        COMMBAD|NOCOMM)
            EMOJI=":x:"
            COLOR="danger"
            ;;
        *)
            EMOJI=":information_source:"
            COLOR="#439FE0"
            ;;
    esac

    # Get UPS details
    CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")
    RUNTIME=$(upsc $UPSNAME battery.runtime 2>/dev/null || echo "N/A")
    [ "$RUNTIME" != "N/A" ] && RUNTIME="$((RUNTIME / 60)) min"

    curl -s -X POST "$SLACK_WEBHOOK" \
        -H 'Content-type: application/json' \
        --data @- << EOF
{
    "channel": "$SLACK_CHANNEL",
    "username": "UPS Monitor",
    "icon_emoji": ":battery:",
    "attachments": [{
        "color": "$COLOR",
        "blocks": [
            {
                "type": "header",
                "text": {
                    "type": "plain_text",
                    "text": "$EMOJI UPS Alert: $NOTIFYTYPE",
                    "emoji": true
                }
            },
            {
                "type": "section",
                "fields": [
                    {"type": "mrkdwn", "text": "*UPS:*\n$UPSNAME"},
                    {"type": "mrkdwn", "text": "*Host:*\n$HOSTNAME"},
                    {"type": "mrkdwn", "text": "*Battery:*\n${CHARGE}%"},
                    {"type": "mrkdwn", "text": "*Runtime:*\n$RUNTIME"}
                ]
            },
            {
                "type": "context",
                "elements": [
                    {"type": "mrkdwn", "text": "$TIMESTAMP"}
                ]
            }
        ]
    }]
}
EOF
}
```

## Discord Notifications

### Create Discord Webhook

1. Open Discord server settings
2. Go to **Integrations** → **Webhooks**
3. Click **New Webhook**
4. Copy webhook URL

### Discord Notification Script

Add to `/etc/nut/notify.conf`:

```bash
# Discord Configuration
DISCORD_WEBHOOK="https://discord.com/api/webhooks/YOUR/WEBHOOK/URL"
```

Add to `/usr/local/bin/nut-notify.sh`:

```bash
send_discord() {
    case "$NOTIFYTYPE" in
        ONLINE)     COLOR=3066993 ;;  # Green
        ONBATT)     COLOR=15105570 ;; # Orange
        LOWBATT|FSD|SHUTDOWN) COLOR=15158332 ;; # Red
        *)          COLOR=3447003 ;;  # Blue
    esac

    CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")
    RUNTIME=$(upsc $UPSNAME battery.runtime 2>/dev/null || echo "N/A")
    [ "$RUNTIME" != "N/A" ] && RUNTIME="$((RUNTIME / 60)) min"

    curl -s -X POST "$DISCORD_WEBHOOK" \
        -H "Content-Type: application/json" \
        --data @- << EOF
{
    "username": "UPS Monitor",
    "avatar_url": "https://cdn-icons-png.flaticon.com/512/3103/3103446.png",
    "embeds": [{
        "title": "🔋 UPS Alert: $NOTIFYTYPE",
        "description": "$MESSAGE",
        "color": $COLOR,
        "fields": [
            {"name": "UPS", "value": "$UPSNAME", "inline": true},
            {"name": "Host", "value": "$HOSTNAME", "inline": true},
            {"name": "Battery", "value": "${CHARGE}%", "inline": true},
            {"name": "Runtime", "value": "$RUNTIME", "inline": true}
        ],
        "footer": {"text": "$TIMESTAMP"}
    }]
}
EOF
}
```

## Pushover Notifications

[Pushover](https://pushover.net/) provides reliable push notifications to iOS/Android.

### Setup

1. Create account at pushover.net
2. Get your **User Key** from dashboard
3. Create an **Application** and get **API Token**

Add to `/etc/nut/notify.conf`:

```bash
# Pushover Configuration
PUSHOVER_TOKEN="your_app_token"
PUSHOVER_USER="your_user_key"
```

Add to `/usr/local/bin/nut-notify.sh`:

```bash
send_pushover() {
    case "$NOTIFYTYPE" in
        LOWBATT|FSD|SHUTDOWN)
            PRIORITY=1
            SOUND="siren"
            ;;
        ONBATT|COMMBAD)
            PRIORITY=0
            SOUND="mechanical"
            ;;
        *)
            PRIORITY=-1
            SOUND="pushover"
            ;;
    esac

    CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")

    curl -s -X POST "https://api.pushover.net/1/messages.json" \
        -d "token=$PUSHOVER_TOKEN" \
        -d "user=$PUSHOVER_USER" \
        -d "title=UPS Alert: $NOTIFYTYPE" \
        -d "message=$UPSNAME - Battery: ${CHARGE}% - $MESSAGE" \
        -d "priority=$PRIORITY" \
        -d "sound=$SOUND"
}
```

## Email Notifications

Add to `/etc/nut/notify.conf`:

```bash
# Email Configuration
EMAIL_TO="admin@example.com"
EMAIL_FROM="ups@example.com"
SMTP_SERVER="smtp.example.com"
```

Add to `/usr/local/bin/nut-notify.sh`:

```bash
send_email() {
    CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")
    RUNTIME=$(upsc $UPSNAME battery.runtime 2>/dev/null || echo "N/A")
    
    SUBJECT="[UPS] $NOTIFYTYPE - $UPSNAME"
    
    cat << EOF | mail -s "$SUBJECT" -r "$EMAIL_FROM" "$EMAIL_TO"
UPS Event Notification
=====================

Event:    $NOTIFYTYPE
UPS:      $UPSNAME
Host:     $HOSTNAME
Time:     $TIMESTAMP

Battery:  ${CHARGE}%
Runtime:  ${RUNTIME}s

Message:  $MESSAGE
EOF
}
```

For more complex email (with SMTP auth), use `msmtp` or `ssmtp`:

```bash
# Install msmtp
sudo apt install msmtp msmtp-mta

# Configure ~/.msmtprc
cat > ~/.msmtprc << EOF
defaults
auth on
tls on

account default
host smtp.gmail.com
port 587
from your-email@gmail.com
user your-email@gmail.com
password your-app-password
EOF
chmod 600 ~/.msmtprc
```

## Telegram Notifications

### Create Telegram Bot

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot` and follow prompts
3. Copy the bot token
4. Start a chat with your bot and send a message
5. Get your chat ID: `curl https://api.telegram.org/bot<TOKEN>/getUpdates`

Add to `/etc/nut/notify.conf`:

```bash
# Telegram Configuration
TELEGRAM_BOT_TOKEN="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
TELEGRAM_CHAT_ID="your_chat_id"
```

Add to `/usr/local/bin/nut-notify.sh`:

```bash
send_telegram() {
    case "$NOTIFYTYPE" in
        ONLINE)     EMOJI="✅" ;;
        ONBATT)     EMOJI="⚠️" ;;
        LOWBATT)    EMOJI="🔴" ;;
        *)          EMOJI="ℹ️" ;;
    esac

    CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")

    MESSAGE_TEXT="$EMOJI *UPS Alert: $NOTIFYTYPE*

🔋 UPS: \`$UPSNAME\`
🖥️ Host: \`$HOSTNAME\`
🔌 Battery: ${CHARGE}%
⏰ Time: $TIMESTAMP"

    curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
        -d "chat_id=$TELEGRAM_CHAT_ID" \
        -d "parse_mode=Markdown" \
        -d "text=$MESSAGE_TEXT"
}
```

## Ntfy Notifications

[Ntfy](https://ntfy.sh/) is a simple, self-hostable notification service.

Add to `/etc/nut/notify.conf`:

```bash
# Ntfy Configuration
NTFY_SERVER="https://ntfy.sh"  # Or your self-hosted instance
NTFY_TOPIC="your-ups-alerts"
# Optional authentication
# NTFY_TOKEN="tk_your_token"
```

Add to `/usr/local/bin/nut-notify.sh`:

```bash
send_ntfy() {
    case "$NOTIFYTYPE" in
        LOWBATT|FSD|SHUTDOWN) PRIORITY="urgent" ;;
        ONBATT|COMMBAD)       PRIORITY="high" ;;
        ONLINE)               PRIORITY="default" ;;
        *)                    PRIORITY="low" ;;
    esac

    CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")

    AUTH_HEADER=""
    [ -n "$NTFY_TOKEN" ] && AUTH_HEADER="-H \"Authorization: Bearer $NTFY_TOKEN\""

    curl -s -X POST "$NTFY_SERVER/$NTFY_TOPIC" \
        -H "Title: UPS Alert: $NOTIFYTYPE" \
        -H "Priority: $PRIORITY" \
        -H "Tags: battery,warning" \
        $AUTH_HEADER \
        -d "UPS: $UPSNAME | Battery: ${CHARGE}% | Host: $HOSTNAME"
}
```

## Using upssched for Timed Alerts

Avoid notification spam by using `upssched` to delay alerts.

### Configure upssched

Edit `/etc/nut/upssched.conf`:

```ini
CMDSCRIPT /usr/local/bin/upssched-cmd
PIPEFN /run/nut/upssched.pipe
LOCKFN /run/nut/upssched.lock

# Wait 30 seconds on battery before alerting (filters short outages)
AT ONBATT * START-TIMER onbatt 30
AT ONLINE * CANCEL-TIMER onbatt

# Immediate notification for critical events
AT LOWBATT * EXECUTE lowbatt
AT FSD * EXECUTE fsd
AT COMMBAD * EXECUTE commbad
AT COMMOK * EXECUTE commok

# Cancel battery timer when back online
AT ONLINE * EXECUTE online
```

Create `/usr/local/bin/upssched-cmd`:

```bash
#!/bin/bash

case "$1" in
    onbatt)
        # Only fires if on battery for 30+ seconds
        /usr/local/bin/nut-notify.sh "ONBATT_SUSTAINED" "$UPSNAME" "UPS on battery for 30+ seconds"
        ;;
    online)
        /usr/local/bin/nut-notify.sh "ONLINE" "$UPSNAME" "Power restored"
        ;;
    lowbatt)
        /usr/local/bin/nut-notify.sh "LOWBATT" "$UPSNAME" "Battery critically low!"
        ;;
    fsd)
        /usr/local/bin/nut-notify.sh "FSD" "$UPSNAME" "Forced shutdown initiated"
        ;;
    commbad)
        /usr/local/bin/nut-notify.sh "COMMBAD" "$UPSNAME" "Lost communication with UPS"
        ;;
    commok)
        /usr/local/bin/nut-notify.sh "COMMOK" "$UPSNAME" "Communication restored"
        ;;
esac
```

Make executable:
```bash
chmod +x /usr/local/bin/upssched-cmd
```

Update `upsmon.conf` to use upssched:
```ini
NOTIFYCMD /usr/sbin/upssched
```

## Home Assistant Integration

For the most flexible notifications, integrate with Home Assistant. See [Smart Device Shutdown](smart-shutdown.md) for full details.

### Basic Home Assistant Notification

```yaml
# automations.yaml
- alias: "UPS on Battery Alert"
  trigger:
    - platform: state
      entity_id: sensor.myups_ups_status
      to: "OB"
  action:
    - service: notify.mobile_app_your_phone
      data:
        title: "⚠️ UPS Alert"
        message: "Power outage detected! Battery: {{ states('sensor.myups_battery_charge') }}%"
        data:
          priority: high
          ttl: 0
```

## Complete Notification Script

Here's a complete `/usr/local/bin/nut-notify.sh`:

```bash
#!/bin/bash
#
# Complete NUT Notification Script
#

NOTIFYTYPE="$1"
UPSNAME="$2"
MESSAGE="${3:-UPS event: $NOTIFYTYPE}"

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
HOSTNAME=$(hostname)

# Source configuration
source /etc/nut/notify.conf 2>/dev/null

# Get UPS stats
CHARGE=$(upsc $UPSNAME battery.charge 2>/dev/null || echo "N/A")
RUNTIME=$(upsc $UPSNAME battery.runtime 2>/dev/null || echo "N/A")
[ "$RUNTIME" != "N/A" ] && RUNTIME_MIN="$((RUNTIME / 60)) min" || RUNTIME_MIN="N/A"

# Log event
logger -t nut-notify "[$NOTIFYTYPE] $UPSNAME: Battery ${CHARGE}% - $MESSAGE"

# Slack
if [ -n "$SLACK_WEBHOOK" ]; then
    case "$NOTIFYTYPE" in
        ONLINE)  COLOR="good" ;;
        ONBATT)  COLOR="warning" ;;
        *)       COLOR="danger" ;;
    esac
    
    curl -s -X POST "$SLACK_WEBHOOK" \
        -H 'Content-type: application/json' \
        -d "{\"text\":\"🔋 *$NOTIFYTYPE* - $UPSNAME\\nBattery: ${CHARGE}% | Runtime: $RUNTIME_MIN | Host: $HOSTNAME\"}" &
fi

# Discord
if [ -n "$DISCORD_WEBHOOK" ]; then
    curl -s -X POST "$DISCORD_WEBHOOK" \
        -H "Content-Type: application/json" \
        -d "{\"content\":\"🔋 **$NOTIFYTYPE** - $UPSNAME\\nBattery: ${CHARGE}% | Runtime: $RUNTIME_MIN | Host: $HOSTNAME\"}" &
fi

# Pushover
if [ -n "$PUSHOVER_TOKEN" ] && [ -n "$PUSHOVER_USER" ]; then
    curl -s -X POST "https://api.pushover.net/1/messages.json" \
        -d "token=$PUSHOVER_TOKEN&user=$PUSHOVER_USER&title=UPS: $NOTIFYTYPE&message=$UPSNAME - ${CHARGE}%" &
fi

# Telegram
if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
    curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
        -d "chat_id=$TELEGRAM_CHAT_ID&text=🔋 $NOTIFYTYPE - $UPSNAME | ${CHARGE}%" &
fi

# Ntfy
if [ -n "$NTFY_TOPIC" ]; then
    curl -s -X POST "${NTFY_SERVER:-https://ntfy.sh}/$NTFY_TOPIC" \
        -H "Title: UPS: $NOTIFYTYPE" \
        -d "$UPSNAME - Battery: ${CHARGE}% - $HOSTNAME" &
fi

wait
exit 0
```

## Testing Notifications

```bash
# Test script directly
/usr/local/bin/nut-notify.sh ONBATT myups "Test notification"

# Test via upsmon (will actually process event)
sudo upsmon -c reload

# Check logs
journalctl -t nut-notify -f
```
