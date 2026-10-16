---
title: homelab-nut version
sidebar_label: version
sidebar_position: 11
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_version
---

# `homelab-nut version`

Print the version, commit, and build date (or JSON with -o json)

## Usage

```
homelab-nut version [flags]
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
