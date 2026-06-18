![homelab-nut](cover.png)

# homelab-nut

**Network UPS Tools, set up from your laptop.**

Wire up [Network UPS Tools (NUT)](https://networkupstools.org/) across a homelab — monitor your UPS, push notifications when power events happen, and gracefully shut everything down when the battery runs low. Built for people running 1–10 machines, not enterprise fleets.

---

## The problem

Getting NUT to coordinate graceful shutdown across more than one machine is a multi-day project: install `nut-server` on the host with the UPS, configure `/etc/nut/*.conf` by hand, install `nut-client` on every other machine, generate SSH keys, write a shutdown script, install a systemd unit, configure the threshold logic, hook up notifications. Most people give up halfway and just hope the UPS lasts long enough.

This repo collapses that to a few commands. Setup scripts handle the bash plumbing today; a modern Go CLI + TUI is in active development to run the whole thing from one machine via SSH.

## What homelab-nut does

- **Sets up NUT** — server, clients, and Prometheus exporter — across your fleet, with `apt`/`systemd` configured the way it should be
- **Coordinates graceful shutdown** — a custom systemd daemon polls battery state and SSHes into your other machines to power them down when battery drops below a threshold
- **Handles device quirks** — per-host shutdown recipes for UniFi gear (Dream Machine, UNAS), NAS appliances, smart TVs — anything that doesn't accept a normal SSH script
- **Sends notifications** — Slack, Discord, Pushover, Telegram, ntfy on power events
- **Ships a monitoring stack** — Docker compose with [nut-webgui](https://github.com/SuperioOne/nut_webgui) for status, `druggeri/nut_exporter` for Prometheus, and an importable Grafana dashboard

## The CLI ([v0.1.0-alpha](https://github.com/rtorcato/homelab-nut/releases/tag/v0.1.0-alpha) — what works today)

Download the binary for your platform from the [latest release](https://github.com/rtorcato/homelab-nut/releases/latest) — or build from source with `make build`.

```bash
# Generate an inventory file interactively (charmbracelet/huh forms)
homelab-nut init

# Inspect the inventory
homelab-nut inventory list
homelab-nut inventory show pi-rack
homelab-nut inventory validate

# Open the full 4-screen Bubble Tea TUI (Dashboard / Hosts / Host / Help)
homelab-nut
```

### Coming next (mid-Phase-2)

```bash
homelab-nut plan        # preview what would change on each host (Terraform-style)
homelab-nut apply       # SSH out, install + configure NUT across the fleet
homelab-nut status      # live UPS dashboard across the fleet
```

Three of five setup roles already work over SSH today (`nut-server`, `nut-client`, `exporter`) — they wrap the existing `scripts/*.sh` and stream their output back live. The remaining two roles (`shutdown-daemon`, `shutdown-target`) plus the `plan`/`apply` subcommands land before the v0.1 Alpha tag becomes feature-complete. The bash scripts keep working in parallel; the CLI's wrap-then-port path means there's no migration cliff.

See [**ROADMAP.md**](ROADMAP.md) for the full plan and [**TODOS.md**](TODOS.md) for live status of open work.

## Quick Start

The simplest path that exists today — bash scripts run directly on the host with the UPS:

```bash
git clone https://github.com/rtorcato/homelab-nut.git
cd homelab-nut

# On the host with the UPS attached (Debian/Ubuntu):
sudo ./scripts/setup-server.sh myups usbhid-ups

# Set up coordinated remote shutdown (interactive wizard):
sudo ./scripts/ups-service.sh

# Check status from anywhere:
./ups-status.sh
```

Need a different path? See [Other setup options](#other-setup-options) below — Docker stack, Prometheus exporter, manual NUT, remote-only clients, etc.

## Status / Scope

**In scope:**
- Turnkey NUT server/client/exporter setup for Debian/Ubuntu
- Multi-node shutdown automation with per-host command overrides
- Notification integrations (Slack/Discord/Pushover/Telegram/ntfy)
- Docker stack for nut-exporter + nut-webgui
- Reference Grafana dashboard and Prometheus scrape config

**Out of scope:**
- Replacing NUT or nut-webgui with a custom implementation
- Enterprise / HA cluster scenarios
- Distro packaging (apt/rpm/etc.)

If you're running 1–10 machines and want them to power down cleanly when the UPS battery runs low, this is for you.

## Documentation

| Document | Description |
|---|---|
| [Server Setup](docs/server-setup.md) | Install and configure NUT server |
| [Client Setup](docs/client-setup.md) | Configure clients to monitor a remote UPS |
| [CLI Reference](docs/cli-reference.md) | NUT command-line tools and usage |
| [Docker Setup](docs/docker-setup.md) | Docker stack: nut-webgui + Prometheus exporter |
| [Notifications](docs/notifications.md) | Slack, Discord, Pushover, Telegram |
| [Smart Shutdown](docs/smart-shutdown.md) | UniFi, LG TVs, NAS via Home Assistant |
| [Prometheus + Grafana](docs/prometheus-grafana.md) | Scrape config + importable dashboard |
| [Roadmap](ROADMAP.md) | Where this is heading |
| [TODOs](TODOS.md) | Live status of open work |
| **[Docs site](https://rtorcato.github.io/homelab-nut/)** | Full reference + auto-generated CLI docs |

---

## Other setup options

<details>
<summary><b>Docker monitoring (nut-webgui + Prometheus exporter)</b></summary>

The Docker stack runs **alongside** a bare-metal `nut-server` (it does not run its own copy to avoid USB/port conflicts). Two services: `nut-exporter` (Prometheus metrics) and `nut-webgui` (status UI).

```bash
cd docker
cp .env.example .env                            # edit: NUT_HOST, UPS_NAME
cp nut-webgui.toml.example nut-webgui.toml      # add a [upsd.<name>] section per server
docker compose up -d
```

- **Web UI** (nut-webgui): http://localhost:9000
- **Prometheus exporter**: http://localhost:9199/ups_metrics

Prometheus and Grafana are intentionally not included — host them elsewhere and point them at this host's exporter. See [docs/prometheus-grafana.md](docs/prometheus-grafana.md) for setup.

</details>

<details>
<summary><b>Bare-metal Prometheus exporter (no Docker)</b></summary>

For low-resource hosts (Pi Zero, Pi Zero 2 W) where Docker is overkill, install `druggeri/nut_exporter` as a hardened systemd service:

```bash
sudo ./scripts/setup-exporter.sh                                   # localhost, no auth
sudo ./scripts/setup-exporter.sh 192.0.2.10 upsmon_remote <pwd>    # remote NUT server
```

Auto-detects architecture (amd64/arm64/arm/386), pulls the latest release, runs as a dedicated unprivileged user under `ProtectSystem=strict` / `NoNewPrivileges`.

Status from anywhere with just curl:
```bash
./scripts/exporter-status.sh http://192.0.2.10:9199 myups
```

</details>

<details>
<summary><b>NUT client only (remote machines)</b></summary>

For a machine that monitors a remote UPS over the network (no UPS attached directly):

```bash
sudo ./scripts/setup-client.sh 192.0.2.10 myups <password>
#                              ^server     ^ups  ^password from setup-server.sh
```

</details>

<details>
<summary><b>Manual NUT setup (any distro)</b></summary>

```bash
# Server
sudo apt install nut
sudo nano /etc/nut/ups.conf       # UPS driver config
sudo nano /etc/nut/upsd.conf      # daemon config
sudo nano /etc/nut/upsd.users     # users + passwords
sudo systemctl enable --now nut-server

# Client
sudo apt install nut-client
sudo nano /etc/nut/nut.conf
sudo nano /etc/nut/upsmon.conf
sudo systemctl enable --now nut-client
```

</details>

<details>
<summary><b>How it fits together</b></summary>

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   UPS Device    │────▶│   NUT Server    │────▶│   NUT Client    │
│  (USB/Serial)   │     │  (nut-server)   │     │   (nut-client)  │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                              │                        │
                        Monitors UPS            Receives status
                        Shares data             Triggers shutdown
```

`homelab-nut` adds a custom daemon on the NUT server host that polls battery state and coordinates SSH-based shutdown across multiple remote nodes (with per-host command overrides for devices that can't host a shutdown script).

</details>

## Supported platforms

Debian / Ubuntu (primary), Raspberry Pi OS, Proxmox VE, TrueNAS, anywhere `apt`/`systemd` work. RHEL/Fedora and Alpine are on the roadmap.

## Project structure

```
homelab-nut/
├── cmd/homelab-nut/         # Go CLI entry point (in development)
├── internal/                # CLI/TUI/inventory packages
├── scripts/                 # Bash setup + shutdown scripts (today's path)
├── docker/                  # Docker stack: nut-exporter + nut-webgui
├── docs/                    # User-facing docs
├── examples/                # Inventory examples + Grafana dashboard
├── config/                  # Daemon config (host-specific, gitignored)
├── ROADMAP.md
├── TODOS.md
└── CONTRIBUTING.md
```

## Resources

- [NUT Documentation](https://networkupstools.org/docs/user-manual.chunked/index.html)
- [NUT Hardware Compatibility](https://networkupstools.org/stable-hcl.html)
- [nut-webgui](https://github.com/SuperioOne/nut_webgui) — the web UI used by the Docker stack
- [druggeri/nut_exporter](https://github.com/DRuggeri/nut_exporter) — Prometheus exporter

## Contributing

PRs welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for scope, shellcheck setup, and how to add new per-node shutdown recipes. Be kind: [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
