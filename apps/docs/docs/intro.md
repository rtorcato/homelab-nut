---
sidebar_position: 1
title: Getting Started
---

# Getting Started

`homelab-nut` is a CLI + TUI for wiring up [Network UPS Tools (NUT)](https://networkupstools.org/) across a homelab — monitor your UPS, push notifications when power events happen, and gracefully shut everything down when the battery runs low.

:::info Status
The Go CLI is in **beta** ([v0.2.0-beta](https://github.com/rtorcato/homelab-nut/releases/tag/v0.2.0-beta)). You can define and inspect an inventory, and `homelab-nut apply` sets up NUT across your fleet over SSH. See the [Roadmap](./roadmap.md) for status.
:::

## Install

Download the binary for your platform from the [newest release](https://github.com/rtorcato/homelab-nut/releases) and unpack it on `$PATH`:

```bash
OS=$(uname -s | tr A-Z a-z); ARCH=$(uname -m); [ "$ARCH" = "aarch64" ] && ARCH=arm64
# Resolve the newest release tag (pre-1.0 builds are pre-releases, which the
# /releases/latest endpoint skips — so pull the tag from the API instead).
TAG=$(curl -fsSL https://api.github.com/repos/rtorcato/homelab-nut/releases \
  | grep -m1 '"tag_name"' | cut -d'"' -f4); VER=${TAG#v}
curl -fL "https://github.com/rtorcato/homelab-nut/releases/download/${TAG}/homelab-nut_${VER}_${OS}_${ARCH}.tar.gz" \
  | tar -xz homelab-nut
sudo install homelab-nut /usr/local/bin/
homelab-nut version
```

Homebrew tap and `install.sh` are coming in [Phase 4 (#5)](https://github.com/rtorcato/homelab-nut/issues/5).

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

The bash scripts in [`scripts/`](https://github.com/rtorcato/homelab-nut/tree/main/scripts) do the actual NUT setup on each machine, and the CLI's roles wrap them. The Go CLI handles inventory definition, inspection, and remote setup over SSH via `homelab-nut apply` (since v0.2.0-beta).

The [main README](https://github.com/rtorcato/homelab-nut#quick-start) has the current bash-based Quick Start.
