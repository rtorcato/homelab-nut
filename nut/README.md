# NUT — Network UPS Tools

Monitor a UPS and trigger graceful shutdowns on power failure.

## Files

| File | Purpose |
|---|---|
| `setup.sh` | Automated install + config |
| `README.md` | Manual setup guide (this file) |
| `ups-status.sh` | Pretty-printed UPS status — run on the Pi |
| `upsc-output.md` | Reference for `upsc` output variables |
| `docker/compose.yml` | Prometheus + Grafana stack to monitor UPS from another machine |
| `docker/nut-exporter.toml` | NUT exporter config (host, UPS name, credentials) |
| `docker/prometheus.yml` | Prometheus scrape config |
| `docker/grafana/provisioning/` | Auto-provisions Grafana datasource + dashboard on first boot |
| `docker/grafana/dashboards/` | Place exported dashboard JSON here |

---

## Automated setup

```bash
./nut/setup.sh --dry-run   # preview
./nut/setup.sh             # install + configure
```

After running, complete the steps in the [Post-install](#post-install) section below.

---

## Manual setup

### 1. Install

```bash
sudo apt-get update
sudo apt-get install -y nut nut-client
```

### 2. Detect your UPS

Plug in your UPS via USB, then scan:

```bash
sudo nut-scanner -U
```

Note the `driver`, `vendorid`, and `productid` values — you'll need them in step 4.

**Common drivers:**

| Driver | UPS type |
|---|---|
| `usbhid-ups` | Most modern USB UPS (APC, Eaton, CyberPower, etc.) |
| `blazer_usb` | Many budget USB UPS brands |
| `apcsmart` | APC via serial port |
| `snmp-ups` | Network-attached UPS |

### 3. Configure `/etc/nut/nut.conf`

```
MODE=standalone
```

> **Important:** `nut.conf` must contain *only* `MODE=standalone` — no comments, no extra lines. Any extra content causes `upsd` to report "disabled" and refuse to start.

Use `netserver` instead of `standalone` if other machines will also monitor this UPS.

### 4. Configure `/etc/nut/ups.conf`

Copy `driver`, `vendorid`, and `productid` directly from `nut-scanner -U` output:

```ini
[myups]
    driver = usbhid-ups
    port = auto
    vendorid = 051D
    productid = 0002
    desc = "APC Back-UPS ES 650G1"
```

### 5. Configure `/etc/nut/upsd.conf`

```ini
# Localhost only — change to 0.0.0.0 to allow other machines to connect
LISTEN 127.0.0.1 3493
```

If serving other machines (e.g. a Docker monitoring stack on another host), use:

```ini
LISTEN 0.0.0.0 3493
```

And allow the port through UFW (LAN only):

```bash
sudo ufw allow from 192.0.2.0/24 to any port 3493 proto tcp
```

### 6. Configure `/etc/nut/upsd.users`

```ini
[admin]
    password = yourpassword
    actions = SET
    instcmds = ALL

[monitor]
    password = monpass
    upsmon master

[dockermon]
    password = somepassword
    upsmon slave
```

> Add `dockermon` (or any named user) if a Docker client app will connect remotely. Use `upsmon slave` for clients — `master` is only for the machine directly connected to the UPS.

```bash
sudo chmod 640 /etc/nut/upsd.users
```

### 7. Configure `/etc/nut/upsmon.conf`

```ini
MONITOR myups@localhost 1 monitor monpass master

MINSUPPLIES 1
SHUTDOWNCMD "/sbin/shutdown -h now"
POWERDOWNFLAG /etc/killpower
POLLFREQ 5
POLLFREQALERT 5
DEADTIME 15
FINALDELAY 5
```

```bash
sudo chmod 640 /etc/nut/upsmon.conf
```

### 8. Enable and start services

```bash
sudo systemctl enable nut-server nut-monitor
sudo systemctl start nut-server nut-monitor
```

---

## ups-status.sh

A pretty-printed status summary — run this on the Pi.

```bash
# Default — queries myups@localhost
./nut/ups-status.sh

# Query a specific UPS or remote host
./nut/ups-status.sh myups@localhost
```

Output shows status (Online / On Battery / Low Battery), battery charge, estimated runtime, load, and input/output voltage — colour-coded by severity.

### Getting the script onto the Pi

**Option 1 — copy it directly (quick):**
```bash
# From your Mac, in the dotfiles root
scp nut/ups-status.sh myuser@192.0.2.10:~/ups-status.sh
ssh myuser@192.0.2.10 'chmod +x ~/ups-status.sh && ~/ups-status.sh'
```

**Option 2 — clone the full dotfiles repo on the Pi (permanent):**
```bash
# On the Pi
git clone git@gitlab.com:<your-repo> ~/dotfiles
chmod +x ~/dotfiles/nut/ups-status.sh
~/dotfiles/nut/ups-status.sh
```

To run it from your Mac after copying:
```bash
ssh myuser@192.0.2.10 '~/ups-status.sh'
# or with a specific UPS/host:
ssh myuser@192.0.2.10 '~/ups-status.sh myups@localhost'
```

---

## Post-install

Verify everything is working:

```bash
sudo upsc myups              # show UPS status and variables
sudo upscmd -l myups         # list available commands
systemctl status nut-server  # check service health
```

Key variables to check:

| Variable | Meaning |
|---|---|
| `ups.status` | `OL` = on line, `OB` = on battery, `LB` = low battery |
| `battery.charge` | Battery charge percentage |
| `battery.runtime` | Estimated runtime in seconds |
| `ups.load` | Current load percentage |

See `upsc-output.md` for the full variable reference.

---

## Docker monitoring stack (Prometheus + Grafana)

Runs on your Mac (or any machine on the LAN) and connects to the Pi's NUT server.

### Prerequisites on the Pi

- `upsd.conf` must use `LISTEN 0.0.0.0 3493`
- `upsd.users` must have a `upsmon slave` user (e.g. `dockermon`)
- UFW must allow port 3493 from your LAN

Verify the Pi is reachable:
```bash
nc -zv 192.0.2.10 3493    # should say "succeeded"
```

### Configure

Edit `docker/nut-exporter.toml` — set host, UPS name, and credentials to match `upsd.users`:

```toml
[[ups]]
name = "myups"
host = "192.0.2.10"
port = 3493
username = "dockermon"
password = "somepassword"
```

### Start

```bash
cd nut/docker
docker compose up -d
```

| Service | URL |
|---|---|
| Grafana | http://localhost:3000 (admin / admin) |
| Prometheus | http://localhost:9090 |
| NUT exporter metrics | http://localhost:9995/nut?target=192.0.2.10 |

### Dashboard

The Grafana datasource is provisioned automatically. To provision the dashboard:

1. In Grafana, open your dashboard → **Share → Export → Save to file**
2. Save as `docker/grafana/dashboards/nut.json`
3. Recreate Grafana: `docker compose up -d --force-recreate grafana`

Alternatively, import community dashboard ID **14371** manually via Dashboards → New → Import.

---

## Troubleshooting

**`upsd disabled` on service start**
```bash
cat /etc/nut/nut.conf    # must contain only: MODE=standalone
```

**NUT scanner finds nothing**
```bash
lsusb                    # confirm UPS is visible to the kernel
sudo nut-scanner -U      # rescan — try a different USB cable/port if missing
```

**Driver fails to start**
```bash
sudo upsdrvctl start     # run manually to see the error
sudo dmesg | grep -i usb
```

**Docker exporter can't reach Pi**
```bash
nc -zv 192.0.2.10 3493                        # test connectivity
cat /etc/nut/upsd.conf                         # confirm LISTEN 0.0.0.0 3493
sudo systemctl status nut-server               # confirm service is running
sudo ufw status                                # confirm port 3493 is allowed
```

**Service won't start after config change**
```bash
sudo systemctl restart nut-server nut-monitor
journalctl -u nut-server -n 50
journalctl -u nut-monitor -n 50
```
