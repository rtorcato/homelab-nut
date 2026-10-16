---
title: homelab-nut inventory validate
sidebar_label: validate
sidebar_position: 4
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_inventory_validate
---

# `homelab-nut inventory validate`

Check the inventory against the schema and rules

## Usage

```
homelab-nut inventory validate [flags]
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
