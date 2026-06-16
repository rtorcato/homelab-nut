# Contributing to homelab-nut

Thanks for considering a contribution. This project is aimed at homelab and small-lab operators running NUT (Network UPS Tools) across a handful of machines.

## What's in scope

- Improving the NUT setup, client, and exporter scripts
- New per-node shutdown recipes (UniFi, Synology, TrueNAS, LG TVs, etc.) — see [Per-node command overrides](config/README.md#per-node-command-overrides-cmd_hostname)
- Better docs, screenshots, examples
- nut-webgui / Prometheus / Grafana integration tips
- Notification integrations (Slack, Discord, Pushover, Telegram, ntfy, etc.)

## Out of scope

- Replacing NUT or nut-webgui with a custom implementation
- Enterprise / HA features that need a real cluster manager
- Per-distro packaging beyond Debian/Ubuntu (PRs welcome but not a priority)

## Before you open a PR

1. **Test the scripts locally.** Most of this is bash — there's no CI substitute for actually running `setup-server.sh` on a fresh VM or `ups-service.sh` against a real UPS.
2. **Run shellcheck.** CI runs it on every push:
   ```bash
   shellcheck scripts/**/*.sh ups-status.sh
   ```
3. **No host-specific values.** Use generic placeholders in docs and examples:
   - IPs: `192.0.2.10` (RFC 5737 documentation range)
   - Hosts: `myhost`, `control-node`, `pi-rack`
   - Users: `myuser`, `admin`
4. **Host-specific configs stay out of git.** `config/*.conf` and `docker/nut-webgui.toml` are gitignored — only the `.example` templates are tracked.
5. **TODOs link to issues.** Any `TODO` or `FIXME` in code must reference a GitHub issue — see [TODOs link to issues](#todos-link-to-issues) below.

## TODOs link to issues

Every `TODO` and `FIXME` comment in source code must reference a GitHub issue. CI enforces this via [`.github/workflows/todo-check.yml`](.github/workflows/todo-check.yml).

**Format:**

```go
// TODO(#42): handle multi-UPS hosts where one driver reports multiple UPS
```

```bash
# TODO(#42): handle multi-UPS hosts
```

```python
# FIXME(#108): race condition when two daemons start simultaneously
```

**Why this rule:**

- Comments rot — issues survive. Six months from now nobody remembers why a `TODO` was left.
- A linked issue is a real conversation: design notes, who's working on it, how it interacts with other work.
- Contributors browsing the code can click through and find work they can pick up.

**What gets checked:**

The CI rule scans tracked files matching `*.sh`, `*.go`, `*.py`, `*.js`, `*.ts`, `*.yaml`, `*.yml`, `*.toml` for the bare words `TODO` or `FIXME`. A match is valid only if immediately followed by `(#N)`. Markdown files are not checked — docs (including this one) often *describe* the pattern.

**If you genuinely want a comment without an issue,** use a different word: `NOTE`, `XXX`, or just a regular comment. Those signal "not action-able" rather than "deferred work."

## Proposing a new shutdown recipe

Per-node shutdown commands live in `config/ups-battery-shutdown.<hostname>.conf` as `CMD_<hostname>=...` overrides. If you've figured out how to gracefully shut down a device that doesn't accept a normal SSH script (UniFi, NAS appliances, smart TVs, etc.), please document it:

1. Add a brief section to [`config/README.md`](config/README.md) under "Per-node command overrides"
2. Mention any prerequisites (SSH key setup, sudo rules, firmware quirks)
3. Open a PR with the title `recipe: <device>`

## Commit style

Short imperative subject lines. Match what's already in `git log`. Reference issues with `#NNN` when relevant.

## Reporting bugs

Open an issue with:
- UPS model and NUT version (`upsc -V`)
- OS and arch (`uname -a`)
- Which script(s) you ran
- What you expected vs. what happened
- Relevant logs (`journalctl -u nut-server`, `journalctl -u ups-battery-shutdown`)

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Be kind, assume good intent.
