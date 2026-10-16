---
title: homelab-nut
sidebar_label: Overview
sidebar_position: 0
slug: /cli/homelab-nut
---

# `homelab-nut`

Modern CLI + TUI for Network UPS Tools (NUT)

homelab-nut sets up and operates a NUT-based UPS fleet from your laptop.

Run with no arguments to open the interactive TUI. Subcommands like
`init`, `plan`, `apply`, `status` are exposed for scripted use.

See https://github.com/rtorcato/homelab-nut/blob/main/ROADMAP.md

## Usage

```
homelab-nut
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [apply](./homelab-nut_apply)
- [backup](./homelab-nut_backup)
- [detect](./homelab-nut_detect)
- [init](./homelab-nut_init)
- [inventory](./homelab-nut_inventory)
- [logs](./homelab-nut_logs)
- [plan](./homelab-nut_plan)
- [shutdown](./homelab-nut_shutdown)
- [status](./homelab-nut_status)
- [uninstall](./homelab-nut_uninstall)
- [version](./homelab-nut_version)
