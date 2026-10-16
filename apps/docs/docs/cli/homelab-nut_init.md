---
title: homelab-nut init
sidebar_label: init
sidebar_position: 4
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_init
---

# `homelab-nut init`

Generate homelab-nut.yaml interactively

Walks through a guided form, then writes the resulting inventory.
With no --inventory/-i flag it writes ./homelab-nut.yaml when that file
already exists, otherwise ~/homelab-nut.yaml.

If an inventory already exists, you'll be asked whether to overwrite it.

This subcommand and the TUI's empty-state 'i' shortcut run the same forms —
use whichever fits your context.

## Usage

```
homelab-nut init [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--no-apply` |  | `false` | write the inventory but don't apply it (generate only — run apply later) |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
