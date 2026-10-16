---
title: homelab-nut detect
sidebar_label: detect
sidebar_position: 3
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_detect
---

# `homelab-nut detect`

Scan nut-server host(s) for connected UPS hardware (nut-scanner)

Connects over SSH to a nut-server host and runs nut-scanner to detect the
connected UPS and the driver NUT should use for it. With no argument, every
host with the nut-server role is scanned.

By default detect is read-only: it reports what's connected and the
recommended driver. Pass --write to persist the detected driver into the
inventory (only when exactly one UPS is found, to avoid guessing).

nut-scanner ships with the NUT package, which 'homelab-nut apply' installs —
so a brand-new host must be applied once before detect can see anything.

With -o json, output is a JSON array suitable for AI agents and scripts.

## Usage

```
homelab-nut detect [host] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |
| `--write` |  | `false` | persist the detected driver into the inventory (single-UPS hosts only) |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
