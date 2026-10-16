---
title: homelab-nut apply
sidebar_label: apply
sidebar_position: 1
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_apply
---

# `homelab-nut apply`

Plan + execute changes on each host over SSH

Loads the inventory, plans what would change on each host, prompts
for confirmation (unless --auto-approve), then executes Apply for each
role with a non-empty diff. Streams each role's output prefixed with
[host/role] so concurrent fleet output stays attributable.

## Usage

```
homelab-nut apply [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-y, --auto-approve` |  | `false` | skip the y/N confirmation prompt before applying |
| `--concurrency` | `int` | `0` | max hosts to apply against in parallel (0 = unlimited) |
| `--host` | `string` | — | apply only this host (the full inventory is still used to resolve cross-host roles) |
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
