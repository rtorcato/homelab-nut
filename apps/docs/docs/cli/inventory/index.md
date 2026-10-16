---
title: homelab-nut inventory
sidebar_label: inventory
sidebar_position: 0
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_inventory
---

# `homelab-nut inventory`

Inspect and edit the homelab-nut inventory

The inventory is a YAML file describing the hosts in your homelab,
their roles, and the UPS attachments. With no --inventory/-i flag,
homelab-nut uses ./homelab-nut.yaml if present, otherwise
~/homelab-nut.yaml.

Subcommands let you list hosts, validate the file, show a single host,
or open the file in your $EDITOR (re-validating on save).

## Usage

```
homelab-nut inventory [command]
```

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
- [inventory edit](./homelab-nut_inventory_edit)
- [inventory list](./homelab-nut_inventory_list)
- [inventory show](./homelab-nut_inventory_show)
- [inventory validate](./homelab-nut_inventory_validate)
