---
title: homelab-nut shutdown
sidebar_label: shutdown
sidebar_position: 0
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_shutdown
---

# `homelab-nut shutdown`

Operate the fleet's shutdown chain

Commands for the battery-shutdown chain. Today this is just 'test', a
dry-run that verifies each target can be reached and its shutdown command
resolved — without powering anything off.

## Usage

```
homelab-nut shutdown [command]
```

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
- [shutdown test](./homelab-nut_shutdown_test)
