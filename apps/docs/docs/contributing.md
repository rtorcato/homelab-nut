---
sidebar_position: 4
title: Contributing
---

# Contributing

The canonical contributing guide is [CONTRIBUTING.md](https://github.com/rtorcato/homelab-nut/blob/main/CONTRIBUTING.md) in the repo. This page is a quick summary; please read that one for the full policy.

## In scope

- Improving the NUT setup, client, and exporter scripts
- New per-host shutdown recipes (UniFi, Synology, TrueNAS, LG TVs, …)
- The Go CLI + TUI (Cobra subcommands, Bubble Tea screens)
- The SSH executor + role implementations
- Better docs, screenshots, examples
- Notification integrations

## Out of scope

- Replacing NUT or nut-webgui with a custom implementation
- Enterprise / HA features
- Per-distro packaging beyond Debian/Ubuntu (PRs welcome but not a priority)

## Before you open a PR

1. **Test locally.** Most setup happens on real machines — there's no CI substitute for running `setup-server.sh` on a fresh VM or driving the daemon against a real UPS.
2. **Run the linters:**
   - Go: `make lint` (golangci-lint)
   - Bash: `shellcheck scripts/**/*.sh ups-status.sh`
   - Docs site: `pnpm --dir apps/docs lint`
3. **No host-specific values.** Use generic placeholders in docs and examples:
   - IPs: `192.0.2.10` (RFC 5737 documentation range)
   - Hosts: `myhost`, `control-node`, `pi-rack`
   - Users: `myuser`, `admin`
4. **TODOs link to issues.** Any `TODO` or `FIXME` in code must reference a GitHub issue:
   ```go
   // TODO(#42): handle multi-UPS hosts
   ```
   CI ([`.github/workflows/todo-check.yml`](https://github.com/rtorcato/homelab-nut/blob/main/.github/workflows/todo-check.yml)) enforces this.

## How work is tracked

- [ROADMAP.md](https://github.com/rtorcato/homelab-nut/blob/main/ROADMAP.md) — vision, phases, milestones
- [TODOS.md](https://github.com/rtorcato/homelab-nut/blob/main/TODOS.md) — live index of open issues (auto-synced)
- [Issues](https://github.com/rtorcato/homelab-nut/issues) — grouped by `phase-N` labels; filter by [`good first issue`](https://github.com/rtorcato/homelab-nut/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) if you're new

## Code of Conduct

This project follows the [Contributor Covenant](https://github.com/rtorcato/homelab-nut/blob/main/CODE_OF_CONDUCT.md). Be kind, assume good intent.
