# Slack Notifications

The UPS daemon can post to a Slack channel when a power event occurs. Notifications are sent from the Pi — no agent needed on the remote nodes.

## Events

| Event | Message |
|---|---|
| Battery hits threshold | `:warning: homelab-nut — Battery at 42% (threshold: 50%). Shutting down: myuser@control-node` |
| SSH to a node fails | `:x: homelab-nut — Failed to SSH to control-node (exit 255)` |
| Power restored | `:white_check_mark: homelab-nut — Power restored. UPS back on mains.` |

---

## Setup

### 1. Create a Slack app and webhook

1. Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From scratch**
2. Name it (e.g. `homelab-nut`) and select your workspace → **Create App**
3. In the left sidebar → **Incoming Webhooks** → toggle **On**
4. Click **Add New Webhook to Workspace**
5. Pick the channel where alerts should appear → **Allow**
6. Copy the webhook URL — it looks like:
   ```
   https://hooks.slack.com/services/T.../B.../...
   ```

### 2. Add the webhook to the Pi config

```bash
sudo nano /etc/ups-battery-shutdown.conf
```

Add this line (replace with your actual URL):

```bash
SLACK_WEBHOOK=https://hooks.slack.com/services/T.../B.../...
```

### 3. Restart the service

```bash
sudo systemctl restart ups-battery-shutdown
```

### 4. Test it

Trigger a dry-run to confirm the webhook is reachable:

```bash
curl -s -X POST "$SLACK_WEBHOOK" \
  -H 'Content-type: application/json' \
  -d '{"text": ":test_tube: UPS notification test from homelab-nut"}' 
```

You should see the message appear in your Slack channel within a few seconds.

---

## Notes

- If `SLACK_WEBHOOK` is not set in the config, notifications are silently skipped — no error.
- A failed webhook (e.g. no internet during outage) will not block or delay the shutdown sequence.
- Notifications are sent by the Pi daemon (`battery-shutdown.sh`). The remote node's `shutdown.sh` does not send Slack messages.
- The webhook URL is stored in `/etc/ups-battery-shutdown.conf` (mode `640`, root-owned). Keep it out of git — the config file is already gitignored by hostname.
