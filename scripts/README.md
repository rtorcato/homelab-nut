# 📜 scripts/

Scripts for setting up and managing NUT on the Pi. Run these on the **Pi** unless noted otherwise.

---

## 🔧 Setup

### `setup-server.sh`
Installs and configures NUT server (`nut-server`, `nut-driver`, `upsmon`) on Debian/Ubuntu. Detects the UPS via USB, writes all config files under `/etc/nut/`, generates credentials, and starts the services.

```bash
sudo ./setup-server.sh                        # auto-detect UPS name + driver
sudo ./setup-server.sh myups usbhid-ups       # explicit UPS name and driver
```

---

### `setup-client.sh`
Configures NUT client mode on a machine that monitors a **remote** NUT server (no UPS physically attached). Sets up `upsmon` to poll the server over the network.

```bash
sudo ./setup-client.sh 192.0.2.10 myups monpass
#                      ^SERVER_IP  ^UPS   ^PASSWORD
```

---

### `setup-exporter.sh`
Installs [`druggeri/nut_exporter`](https://github.com/DRuggeri/nut_exporter) as a bare-metal systemd service (no Docker). Use this on low-resource hosts (Pi Zero etc.) instead of the Docker stack in `../docker/`.

```bash
sudo ./setup-exporter.sh                                  # scrape localhost, no auth
sudo ./setup-exporter.sh 192.168.1.10 upsmon secret       # scrape remote NUT server

# Pin a version or override port:
NUT_EXPORTER_VERSION=3.0.0 sudo ./setup-exporter.sh
NUT_EXPORTER_PORT=9200      sudo ./setup-exporter.sh
```

---

## 📊 Monitoring

### `exporter-status.sh`
Pulls UPS metrics from a running `nut_exporter` HTTP endpoint and renders a status summary — no `upsc` needed. Run this from **any machine** on the network.

```bash
./exporter-status.sh                                      # uses EXPORTER_URL env or prompts
./exporter-status.sh http://192.0.2.10:9199 myups
./exporter-status.sh http://pi:9199 myups --json          # JSON output
./exporter-status.sh http://pi:9199 myups --raw           # dump all raw metric lines
```

---

## 🔋 Battery

### `test-battery.sh`
Triggers a battery self-test on the UPS attached to this host. Must be run locally — refuses remote UPS targets.

```bash
./test-battery.sh                             # quick test (default)
./test-battery.sh myups --quick -y            # skip confirmation
./test-battery.sh myups --deep                # full discharge test
./test-battery.sh myups --status              # show last test result
./test-battery.sh myups --stop                # abort a running test
```

Credentials are read from `NUT_USER`/`NUT_PASS` env vars, `/root/nut-credentials.txt`, or prompted interactively.

---

## 🔑 Credentials

### `show-credentials.sh`
Prints the NUT credentials stored in `/root/nut-credentials.txt` (written by `setup-server.sh`).

```bash
./show-credentials.sh
```

---

## ⚡ Remote Shutdown Service

### `ups-service.sh`
Sets up and manages the `ups-battery-shutdown` systemd service. On first run it launches an interactive wizard (SSH keys, remote node, threshold). Subsequent runs open a management menu.

```bash
sudo ./ups-service.sh                         # wizard on first run, menu after
sudo ./ups-service.sh status                  # show service state + live battery
sudo ./ups-service.sh start | stop | restart
sudo ./ups-service.sh set-threshold 40        # change battery % trigger
sudo ./ups-service.sh edit                    # open config in $EDITOR
sudo ./ups-service.sh logs                    # tail service logs
sudo ./ups-service.sh setup                   # re-run the setup wizard
sudo ./ups-service.sh remove                  # uninstall the service
```

Config is stored at [`../config/ups-battery-shutdown.conf`](../config/) and symlinked to `/etc/ups-battery-shutdown.conf`.

> The underlying daemon (`services/battery-shutdown.sh`) is installed automatically — see [`services/README.md`](services/README.md).
