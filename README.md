![homelab-nut](cover.png)

# homelab-nut

**Network UPS Tools, set up from your laptop.** One CLI to wire NUT across a homelab — server, clients, Prometheus exporter, and multi-node graceful shutdown — from a TUI for humans or `-o json` subcommands for scripts and AI agents.

## Install

Linux + macOS, x86_64 + arm64. Grab the [latest release](https://github.com/rtorcato/homelab-nut/releases/latest):

```bash
OS=$(uname -s | tr A-Z a-z); ARCH=$(uname -m); [ "$ARCH" = "aarch64" ] && ARCH=arm64
curl -L "https://github.com/rtorcato/homelab-nut/releases/latest/download/homelab-nut_*_${OS}_${ARCH}.tar.gz" \
  | tar -xz homelab-nut
sudo install homelab-nut /usr/local/bin/
homelab-nut version
```

Homebrew tap + one-line `install.sh` land in [Phase 4 (#5)](https://github.com/rtorcato/homelab-nut/issues/5).

### Build or run from source (development)

Go 1.25+ required (`brew install go`).

```bash
make run                      # build bin/homelab-nut and launch the TUI
make build                    # just compile -> bin/homelab-nut (versioned)
./bin/homelab-nut             # run the built binary

# Run straight from source without building a binary:
go run ./cmd/homelab-nut                                                # TUI
go run ./cmd/homelab-nut -i examples/inventories/full-homelab.yaml inventory list    # a subcommand
```

`make help` lists every dev target (`test`, `lint`, `docs-dev`, …). See **[CLAUDE.md](CLAUDE.md)** for the full build/test/lint reference and contributor workflow.

## Run it

```bash
homelab-nut
```

That opens the TUI. From an empty directory it walks you through setup — press `i` to generate `homelab-nut.yaml` with guided forms, `e` to edit it in `$EDITOR`, `a` to apply changes over SSH, `?` for the full keymap, `o` to open this project page.

With an inventory already in place:

```text
$ homelab-nut -i examples/inventories/full-homelab.yaml inventory list
NAME      ADDRESS     USER   ROLES
rack-pi   192.0.2.10  pi     nut-server,exporter,shutdown-daemon
nas-host  192.0.2.11  admin  nut-server,exporter,shutdown-daemon
desktop   192.0.2.20  admin  nut-client
nas       192.0.2.30  root   shutdown-target
gateway   192.0.2.1   root   shutdown-target
```

New to the schema? [`examples/inventories/`](examples/inventories/) has commented sample configs for common topologies — minimal single-server, server + network client, SSH shutdown-targets, and a full multi-UPS homelab.

For AI agents, CI, and scripts — every subcommand emits stable JSON with documented exit codes:

```bash
homelab-nut init                              # interactive bootstrap (charmbracelet/huh)
homelab-nut inventory list -o json | jq .     # array of host objects
homelab-nut inventory validate -o json        # { "valid": bool, "errors": [...] }
homelab-nut plan -o json                      # dry-run, full diff tree
homelab-nut apply --auto-approve -o json      # execute, summary as JSON
homelab-nut status -o json                    # live UPS state via NUT protocol (port 3493)
homelab-nut status --watch                    # text dashboard, refreshes every 5s
homelab-nut version -o json                   # { "version", "commit", "date" }
```

Full agent contract in **[AGENTS.md](AGENTS.md)** — common flows, JSON schemas per subcommand, exit-code semantics, what NOT to invoke.

## Demo

> **Asciinema cast + TUI screenshots:** coming in [Phase 4 (#5)](https://github.com/rtorcato/homelab-nut/issues/5) alongside Homebrew packaging. Until then, the [docs site](https://rtorcato.github.io/homelab-nut/) carries an auto-generated CLI reference rendered from cobra.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Validation / config error (user-fixable) |
| `2` | Network / SSH error (transient — retry-safe) |
| `3` | Apply partial failure (some hosts OK, some failed) |

## UPS status codes

`status` (CLI and TUI Dashboard) reports the raw NUT `ups.status` string. Multiple tokens combine with spaces — `OL CHRG` means "on line and charging," `OB LB` means "on battery, low battery."

| Token | Meaning | TUI badge |
|---|---|---|
| `OL` | On Line — utility power, healthy | 🟢 green |
| `OL CHRG` | On Line, battery is charging | 🟢 green |
| `OB` | On Battery — utility power out, UPS sustaining | 🟠 amber |
| `OB LB` | On Battery + Low Battery — imminent shutdown | 🔴 red |
| `LB` | Low Battery (regardless of OL/OB) | 🔴 red |
| `RB` | Replace Battery | warn |
| `BYPASS` | Load on raw mains (UPS bypassed) | warn |
| `FSD` | Forced Shutdown initiated | 🔴 red |
| `CHRG` / `DISCHRG` | Charging / discharging modifier | informational |
| `OVER` / `TRIM` / `BOOST` | Overload / voltage trim / voltage boost | warn |
| `OFF` | UPS reports itself offline | warn |
| _(host error)_ | Unreachable / protocol error | 🔴 red `ERR` |
| _(empty)_ | upsd responded but reported no status | ⚫ grey `?` |

Full reference, including the JSON schema for `status -o json`, lives in **[AGENTS.md](AGENTS.md#homelab-nut-status)**.

See **[ROADMAP.md](ROADMAP.md)** for what's coming and **[TODOS.md](TODOS.md)** for live status.

---

## What homelab-nut does

- **Sets up NUT** — server, clients, and Prometheus exporter — across your fleet, with `apt`/`systemd` configured the way it should be
- **Coordinates graceful shutdown** — a custom systemd daemon polls battery state and SSHes into your other machines to power them down when battery drops below a threshold
- **Handles device quirks** — per-host shutdown recipes for UniFi gear (Dream Machine, UNAS), NAS appliances, smart TVs — anything that doesn't accept a normal SSH script
- **Sends notifications** — Slack, Discord, Pushover, Telegram, ntfy on power events
- **Ships a monitoring stack** — Docker compose with [nut-webgui](https://github.com/SuperioOne/nut_webgui) for status, `druggeri/nut_exporter` for Prometheus, and an importable Grafana dashboard

## The problem it solves

Getting NUT to coordinate graceful shutdown across more than one machine is a multi-day project: install `nut-server` on the host with the UPS, configure `/etc/nut/*.conf` by hand, install `nut-client` on every other machine, generate SSH keys, write a shutdown script, install a systemd unit, configure the threshold logic, hook up notifications. Most people give up halfway and just hope the UPS lasts long enough.

`homelab-nut` collapses that to `init` + `apply`. The CLI is the primary path; the underlying bash scripts in [`/scripts/`](scripts/) are still supported for direct use.

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
| [AGENTS.md](AGENTS.md) | LLM-friendly subcommand contract for AI agents + scripts |
| **[Docs site](https://rtorcato.github.io/homelab-nut/)** | Full reference + auto-generated CLI docs |

---

## Direct bash scripts — what `apply` runs underneath

`apply` SSHes into each host and runs the same bash scripts from [`/scripts/`](scripts/). If you'd rather skip the Go binary and run them by hand on the UPS host:

```bash
git clone https://github.com/rtorcato/homelab-nut.git
cd homelab-nut

sudo ./scripts/setup-server.sh myups usbhid-ups   # on the Pi with the UPS
sudo ./scripts/ups-service.sh                     # configure remote shutdown
./ups-status.sh                                   # check UPS state
```

The CLI exists to orchestrate these across a fleet; the scripts themselves remain fully supported. See [Other setup options](#other-setup-options) for the Docker stack, standalone exporter, and manual NUT walkthrough.

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
├── cmd/homelab-nut/         # Go CLI entry point
├── internal/                # CLI/TUI/inventory/roles/ssh/orchestrator packages
├── scripts/                 # Bash setup + shutdown scripts (wrapped by `apply`)
├── docker/                  # Docker stack: nut-exporter + nut-webgui
├── docs/                    # User-facing docs (also published as the docs site)
├── examples/                # Inventory example + Grafana dashboard
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
