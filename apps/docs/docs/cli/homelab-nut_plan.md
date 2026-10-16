---
title: homelab-nut plan
sidebar_label: plan
sidebar_position: 7
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_plan
---

# `homelab-nut plan`

Preview what `apply` would change on each host (or JSON with -o json)

Loads the inventory, opens SSH connections to each host, runs each
role's Detect + Plan, and prints a per-role diff. Read-only — makes
no changes on any host.

Use the same flags as `apply` so you can dry-run with one
command and execute with another.

## Usage

```
homelab-nut plan [flags]
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

- [homelab-nut](./homelab-nut)
