---
title: homelab-nut inventory list
sidebar_label: list
sidebar_position: 2
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_inventory_list
---

# `homelab-nut inventory list`

Print a table of hosts (or JSON with -o json)

## Usage

```
homelab-nut inventory list [flags]
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
