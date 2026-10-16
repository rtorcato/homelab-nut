---
title: homelab-nut status
sidebar_label: status
sidebar_position: 9
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_status
---

# `homelab-nut status`

Poll each nut-server host for live UPS state

Connects to every host in the inventory with the nut-server role over
the NUT TCP protocol (port 3493) and reports its current UPS state:
status string (OL, OB, OB LB, …), battery %, load %, and runtime estimate.

Hosts without the nut-server role are skipped (no live UPS state to read).
With -o json, output is a JSON array suitable for AI agents and scripts.
With --watch, the command redraws every --interval until Ctrl+C; in JSON
mode each tick emits a fresh JSON array on its own line (NDJSON-friendly).

## Usage

```
homelab-nut status [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--interval` | `duration` | `5s` | redraw cadence when --watch is set |
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |
| `--timeout` | `duration` | `2s` | per-host TCP connect + read deadline |
| `-w, --watch` |  | `false` | redraw on an interval instead of exiting (Ctrl+C to stop) |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
