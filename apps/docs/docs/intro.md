---
sidebar_position: 1
title: Getting Started
---

# Getting Started

`homelab-nut` is a CLI + TUI for wiring up [Network UPS Tools (NUT)](https://networkupstools.org/) across a homelab — monitor your UPS, push notifications when power events happen, and gracefully shut everything down when the battery runs low.

:::info Status
The Go CLI is in **alpha** ([v0.1.0-alpha](https://github.com/rtorcato/homelab-nut/releases/tag/v0.1.0-alpha)). You can write and inspect an inventory today; SSH-based fleet apply lands in v0.2 (Phase 2). See the [Roadmap](./roadmap.md) for status.
:::

## Install

Download the binary for your platform from the [latest release](https://github.com/rtorcato/homelab-nut/releases/latest) and unpack it on `$PATH`:

```bash
# macOS / Linux — substitute your arch (x86_64, arm64)
OS=$(uname -s | tr A-Z a-z)
ARCH=$(uname -m); [ "$ARCH" = "aarch64" ] && ARCH=arm64
curl -L "https://github.com/rtorcato/homelab-nut/releases/latest/download/homelab-nut_*_${OS}_${ARCH}.tar.gz" \
  | tar -xz homelab-nut
sudo install homelab-nut /usr/local/bin/
homelab-nut version
```

Homebrew tap and `install.sh` are coming in Phase 4.

## Quick tour

```bash
# Open the interactive TUI (Dashboard, Hosts, Help)
homelab-nut

# Generate an inventory file interactively
homelab-nut init

# Inspect what you've defined
homelab-nut inventory list
homelab-nut inventory show pi-rack
homelab-nut inventory validate
```

See the [CLI Reference](./cli/) for every subcommand.

## What `homelab-nut.yaml` looks like

The inventory describes the machines in your homelab and what each one does:

```yaml
hosts:
  - name: pi-rack
    address: 192.0.2.10
    user: pi
    roles: [nut-server, exporter, shutdown-daemon]
    ups:
      name: myups
      driver: usbhid-ups
  - name: workstation
    address: 192.0.2.20
    user: admin
    roles: [nut-client, shutdown-target]
    shutdown:
      command: ~/shutdown.sh
  - name: dream-machine
    address: 192.0.2.1
    user: admin
    roles: [shutdown-target]
    shutdown:
      command: poweroff

shutdown_daemon:
  threshold: 50
  poll_interval: 30
  slack_webhook_env: SLACK_WEBHOOK
```

Five roles cover the surface:

| Role | What it does |
|---|---|
| `nut-server` | Hosts the UPS via USB/serial, runs `nut-server` + `upsd` |
| `nut-client` | Monitors the UPS over the network via `upsmon` |
| `exporter` | Publishes Prometheus metrics via `nut_exporter` |
| `shutdown-daemon` | Polls battery state and SSHes targets when threshold is hit |
| `shutdown-target` | Receives a shutdown command — script or inline |

## What ships today

The bash scripts in [`scripts/`](https://github.com/rtorcato/homelab-nut/tree/main/scripts) remain the working path for actually setting up NUT on machines. The Go CLI today handles inventory definition + inspection; remote setup via `homelab-nut apply` lands in v0.2.

The [main README](https://github.com/rtorcato/homelab-nut#quick-start) has the current bash-based Quick Start.
