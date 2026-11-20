---
title: homelab-nut shutdown test
sidebar_label: test
sidebar_position: 1
sidebar_class_name: hn-cli
slug: /cli/homelab-nut_shutdown_test
---

# `homelab-nut shutdown test`

Dry-run the shutdown chain on every shutdown-target host (no power-off)

Verifies the shutdown chain works without powering anything off. For each
host with the shutdown-target role, this connects over SSH as the
configured user and checks that the host's shutdown command can be
resolved:

  - a script path (contains '/', e.g. ~/shutdown.sh) must exist and be
    executable (test -x);
  - an inline command (e.g. poweroff) must be found in PATH (command -v).
  - a command starting with sudo (e.g. sudo /sbin/shutdown -h now) must be
    allowed without a password (sudo -n -l <command>).

It prints a host / command / result table. With -o json it emits an array
of {host, command, ok, error}. Exits 3 if any host fails the check.

## Usage

```
homelab-nut shutdown test [flags]
```

## Examples

```
  homelab-nut shutdown test
  homelab-nut shutdown test -o json
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

- [shutdown](./homelab-nut_shutdown)
