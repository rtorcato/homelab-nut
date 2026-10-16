---
title: homelab-nut backup
sidebar_label: backup
sidebar_position: 2
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_backup
---

# `homelab-nut backup`

Snapshot a host's NUT + homelab-nut config to a local tarball

Pulls the files homelab-nut cares about off a target over SSH into a
local gzip tarball: /etc/nut configs, the custom systemd units, a MANIFEST
(NUT / exporter / kernel versions), and a snapshot of the host's inventory
entry. Read-only on the target — no --auto-approve needed.

Secrets (upsd.users password hashes, the exporter's scrape credentials)
are excluded by default so the tarball is safe to share; pass
--include-secrets for a true backup-and-restore capture.

Default path: ./backups/<host>-<timestamp>.tar.gz

## Usage

```
homelab-nut backup [host] [flags]
```

## Examples

```
  homelab-nut backup pi-rack
  homelab-nut backup --all --output /mnt/backups
  homelab-nut backup pi-rack --include-secrets -o json
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` |  | `false` | back up every host in the inventory (required if no host is given) |
| `--concurrency` | `int` | `0` | max hosts to back up in parallel (0 = unlimited) |
| `--include-secrets` |  | `false` | include upsd.users + /etc/default/nut-exporter (sensitive) |
| `-O, --out` | `string` | — | destination directory, or a .tar.gz file path for a single host (default: ./backups) |
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
