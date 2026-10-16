---
title: homelab-nut uninstall
sidebar_label: uninstall
sidebar_position: 10
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_uninstall
---

# `homelab-nut uninstall`

Remove homelab-nut's services + binaries from a host (the inverse of apply)

Reverses what apply installed on a host: stops + removes the custom
systemd units (nut-exporter, ups-battery-shutdown), their binaries, the
config they wrote, and the shutdown-target sudoers rule + script. The
upstream NUT package is left alone unless you pass --purge-nut.

Built on the same orchestrator as apply, so single-host and fleet-wide
(--all) share one code path, with the same --concurrency and the same
--auto-approve / -o json gating. Removals are idempotent — re-running on
an already-clean host is a no-op that reports everything as skipped.

## Usage

```
homelab-nut uninstall [host] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` |  | `false` | uninstall every host in the inventory (required if no host is given) |
| `-y, --auto-approve` |  | `false` | skip the y/N confirmation prompt before removing |
| `--concurrency` | `int` | `0` | max hosts to uninstall in parallel (0 = unlimited) |
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |
| `--purge-nut` |  | `false` | also apt-purge the upstream NUT package and delete /etc/nut (destructive) |
| `--role` | `string` | `all` | limit to one role: nut-server\|nut-client\|exporter\|shutdown-daemon\|shutdown-target\|all |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
