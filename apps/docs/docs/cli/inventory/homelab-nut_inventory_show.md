---
title: homelab-nut inventory show
sidebar_label: show
sidebar_position: 3
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_inventory_show
---

# `homelab-nut inventory show`

Show details for one host (or JSON with -o json)

## Usage

```
homelab-nut inventory show <host> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [inventory](./homelab-nut_inventory)
