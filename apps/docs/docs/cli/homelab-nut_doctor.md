---
title: homelab-nut doctor
sidebar_label: doctor
sidebar_position: 4
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_doctor
---

# `homelab-nut doctor`

Check that shutdown protection works on server, client and daemon hosts

Connects over SSH to every host with the nut-server, nut-client or
shutdown-daemon role and asserts, from that host's own point of view:

  - nut.conf MODE matches the host's roles (a nut-server is not a netclient)
  - upsmon is running and every MONITOR target answers on the NUT port
  - the UPS named in each MONITOR line exists on the server it points at
  - the shutdown daemon is running and can read its configured UPS right now

Unlike 'plan' (diffs against the inventory) and 'status' (nut-server hosts
only), this catches a client monitoring a decommissioned server while
every service still reports "active".

Exits 5 when any check fails or a host can't be probed, so cron and CI can
page someone. With -o json, output is a JSON array of per-host results.

## Usage

```
homelab-nut doctor [host] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --output` | `string` | `text` | output format: text (human) \| json (machine-readable for AI/scripts) |
| `--timeout` | `duration` | `30s` | per-host SSH connect + probe deadline |

## Inherited flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --inventory` | `string` | — | path to the inventory YAML file (default: ./homelab-nut.yaml if present, else ~/homelab-nut.yaml) |

## See also

- [homelab-nut](./homelab-nut)
