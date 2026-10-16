---
title: homelab-nut logs
sidebar_label: logs
sidebar_position: 6
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_logs
---

# `homelab-nut logs`

Tail a host's systemd journal over SSH

Connects to a host over SSH and streams its systemd journal for a NUT
unit — by default the battery-shutdown daemon (ups-battery-shutdown), the
service you most often need when a shutdown misbehaves.

Pass --unit to read any other unit (e.g. nut-server, nut-monitor), --lines
to change how much history is shown, and --follow to stream new entries
live until Ctrl+C.

This is raw journal text, so there's no -o json mode.

## Usage

```
homelab-nut logs <host> [flags]
```

## Examples

```
  homelab-nut logs pi
  homelab-nut logs pi --unit nut-server --lines 500
  homelab-nut logs pi --follow
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --follow` |  | `false` | stream new entries live until Ctrl+C |
| `-n, --lines` | `int` | `200` | number of past journal lines to show |
| `--unit` | `string` | `ups-battery-shutdown` | systemd unit to read |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
