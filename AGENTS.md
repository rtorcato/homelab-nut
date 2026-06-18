# AGENTS.md — using `homelab-nut` from an AI agent or script

This is the bootstrapping contract for any AI tool (Claude, Cursor, Aider, etc.) or automation script operating `homelab-nut`. Read it once; you'll know which commands to invoke and what to expect back.

For humans, the TUI is the front door: run `homelab-nut` and follow the keybindings. This file documents the *subcommand* path, which is the front door for non-humans.

> **Editing the codebase, not just using the tool?** See **[CLAUDE.md](CLAUDE.md)** — repo-development context (build, test, PR workflow, code layout, embed-sync rule, conventions). Different audience, deliberately separate.

## One-line orientation

`homelab-nut` is a Go CLI + TUI that sets up [Network UPS Tools (NUT)](https://networkupstools.org/) across a homelab fleet via SSH. It reads a YAML inventory (`homelab-nut.yaml`), plans changes per host, and applies them by piping tested bash scripts through SSH connections.

**For AI agents and scripts: use the subcommands below with `-o json`. They have stable output shapes and exit codes. Do not pipe TUI output anywhere — the TUI is terminal-only.**

---

## Exit codes (stable contract)

| Code | Meaning | Retry? |
|---|---|---|
| `0` | Success | n/a |
| `1` | Validation / config error (user-fixable: bad YAML, missing field, etc.) | No — fix the input first |
| `2` | Network / SSH error (unreachable host, auth fail, mid-command drop) | Yes — usually transient |
| `3` | Apply partial failure (some hosts OK, some failed) | Maybe — inspect the per-host result; failed hosts may need a fix |

Defined in `internal/cli/exit.go` and locked in by `TestExitCodesAreStable`.

---

## Common flows

### "Set up a homelab from scratch"

```bash
homelab-nut init                              # interactive (huh forms) — writes homelab-nut.yaml
homelab-nut inventory validate -o json        # confirm the file parses
homelab-nut plan -o json                      # dry-run, parse to see proposed changes
homelab-nut apply --auto-approve              # execute. Add `-o json` for a machine-readable summary
```

### "Add a host to an existing fleet"

```bash
homelab-nut inventory edit                    # opens $EDITOR with the YAML
homelab-nut inventory validate -o json        # confirm before applying
homelab-nut plan -o json | jq '.hosts[] | select(.diffs[].actions | length > 0) | .host.name'
homelab-nut apply --auto-approve -o json
```

### "What's the current state?"

```bash
homelab-nut inventory list -o json            # all hosts + their roles
homelab-nut plan -o json                      # full diff tree (Detect ran against each host)
```

### "Bulk-validate inventory across multiple files in CI"

```bash
for f in inventories/*.yaml; do
  if ! homelab-nut -i "$f" inventory validate -o json | jq -e '.valid'; then
    echo "FAIL: $f"; exit 1
  fi
done
```

---

## Subcommand reference

Every subcommand respects the global `--inventory` / `-i` flag (default `./homelab-nut.yaml`) and the `--output` / `-o` flag (default `text`, alternative `json`).

### `homelab-nut init`

Interactive bootstrap of `homelab-nut.yaml`. **Always interactive** — uses `charmbracelet/huh` forms. Don't invoke from non-interactive contexts (no `-o json` mode); use `inventory edit` or write the YAML directly instead.

- **Inputs:** terminal stdin/stdout (interactive only)
- **Outputs:** writes the inventory file at the path from `--inventory`
- **Exit:** 0 on success, 1 if the user aborts

### `homelab-nut inventory list`

```
$ homelab-nut inventory list -o json
[
  {
    "name": "pi-rack",
    "address": "192.0.2.10",
    "user": "pi",
    "roles": ["nut-server", "exporter", "shutdown-daemon"],
    "ups": {"name": "myups", "driver": "usbhid-ups"}
  },
  ...
]
```

- **JSON schema:** array of host objects. Optional fields (`ups`, `shutdown`) are omitted when absent.
- **Exit:** 0 on success, 1 on inventory load/validate failure.

### `homelab-nut inventory show <host>`

```
$ homelab-nut inventory show pi-rack -o json
{
  "name": "pi-rack",
  "address": "192.0.2.10",
  ...
}
```

- **JSON schema:** single host object (same shape as items in `inventory list`).
- **Exit:** 0 if found, 1 if the host name doesn't exist in the inventory.

### `homelab-nut inventory validate`

```
$ homelab-nut inventory validate -o json                # success
{ "valid": true, "path": "homelab-nut.yaml" }

$ homelab-nut inventory validate -o json                # failure
{
  "valid": false,
  "path": "bad.yaml",
  "errors": [
    {"field": "hosts[0].name", "message": "required"},
    {"field": "hosts[0].roles[0]", "message": "unknown role \"bogus\" (valid: ...)"}
  ]
}
```

- **JSON schema:** `{ valid: bool, path: string, errors?: [{field, message}] }`
- **Exit:** 0 if valid, 1 otherwise (even when emitting JSON — non-zero exit is the signal).

### `homelab-nut inventory edit`

Opens `$EDITOR` on the inventory file, re-validates on save. Interactive — no `-o json` mode.

- **Inputs:** `$EDITOR` (defaults to `vi`)
- **Exit:** 0 if save+validate succeeds, 1 if validation fails after edit, non-zero if the editor itself errors.

### `homelab-nut plan`

```
$ homelab-nut plan -o json
{
  "hosts": [
    {
      "host": { "name": "pi-rack", ... },
      "diffs": [
        {
          "host": { ... },
          "role": "nut-server",
          "current": "missing",
          "target": "ok",
          "actions": [
            "install nut-server, nut-driver, nut-client, upsmon (apt)",
            "configure ups.conf for UPS \"myups\" (driver usbhid-ups)",
            ...
          ]
        }
      ],
      "skipped": ["nut-client"]
    }
  ]
}
```

- **JSON schema:** `{ hosts: [{ host, diffs: [{host, role, current, target, actions}], skipped, errors? }] }`
- **State enum:** `"unknown" | "missing" | "partial" | "ok"`
- **Read-only** — opens SSH connections to call `Detect` but doesn't change anything.
- **Exit:** 0 if all detections succeed, 1 if any planning step errors (per-host errors surface in the `errors` field; the command still exits 1).

### `homelab-nut apply`

**Always use `--auto-approve` / `-y` from non-interactive contexts** — the default behaviour is an interactive y/N prompt that will hang. JSON mode actively refuses to start without `--auto-approve` (exit 1 with a clear error).

```
$ homelab-nut apply --auto-approve -o json
{
  "elapsed": "2m13s",
  "hosts_changed": 3,
  "actions": 14,
  "failed": 0,
  "result": { "hosts": [ ... full per-host diffs + errors ... ] }
}
```

- **JSON schema:** `{ elapsed: string, hosts_changed: int, actions: int, failed: int, result: <plan-result-shape> }`
- **No-op case:** `{ "elapsed": "0s", "noop": true }` (nothing was applied).
- **Per-role output:** in text mode, streamed with `[host/role]` prefix as roles run. In JSON mode, role output is captured but not surfaced in the summary (the orchestrator writes to `io.Discard`) — the `result.hosts[].errors` array carries the failure detail.
- **Exit:** 0 success, 1 plan-time validation error, 3 apply partial failure.
- **Flags:** `--auto-approve/-y`, `--concurrency N` (max parallel hosts; 0 = unlimited).

### `homelab-nut version`

```
$ homelab-nut version -o json
{ "version": "v0.2.0-alpha", "commit": "abc1234", "date": "2026-06-18T..." }
```

- **JSON schema:** `{ version, commit, date }`
- **Exit:** always 0.

---

## Inventory schema

Documented end-to-end in [docs/intro.md](docs/intro.md) and the [docs site](https://rtorcato.github.io/homelab-nut/docs/intro). Quick reference for AI agents writing inventory files:

```yaml
hosts:
  - name: pi-rack                # required, unique, no whitespace
    address: 192.0.2.10          # required, IP or DNS-resolvable hostname
    user: pi                     # required, SSH user
    roles:                       # required, non-empty; values from the enum below
      - nut-server
      - exporter
      - shutdown-daemon
    ups:                         # required when host has role `nut-server`
      name: myups
      driver: usbhid-ups
  - name: workstation
    address: 192.0.2.20
    user: admin
    roles: [nut-client, shutdown-target]
    shutdown:                    # used when host has role `shutdown-target`
      command: ~/shutdown.sh     # path → deployed; bare cmd (e.g. `poweroff`) → sent inline
shutdown_daemon:                 # required when any host has role `shutdown-daemon`
  threshold: 50                  # 1-99 (% battery)
  poll_interval: 30              # seconds, > 0
  slack_webhook_env: SLACK_WEBHOOK   # optional; env var name (not URL) at apply time
```

**Role enum:** `nut-server`, `nut-client`, `exporter`, `shutdown-daemon`, `shutdown-target`.

**Validation rules** (enforced by `inventory validate`, also at `plan` and `apply` time):
- Required: `hosts[].name`, `address`, `user`, `roles`
- No duplicate host names
- `nut-server` requires `ups.name` + `ups.driver`
- `shutdown-target` with a `shutdown` block needs `command`
- `shutdown_daemon` block requires at least one host with `shutdown-daemon` role
- `threshold` ∈ [1, 99], `poll_interval` > 0

---

## What NOT to invoke

- **`apply` without `--auto-approve` in non-interactive contexts** — it'll hang on the y/N prompt. Both text and JSON modes refuse to proceed without `-y` when stdin isn't a TTY.
- **`homelab-nut` with no args from a script** — that opens the TUI. From an agent context, use the subcommand you actually want.
- **`init` from a non-interactive context** — it's huh forms; no flags can drive it. Write the YAML directly or use `inventory edit` with `$EDITOR=cat` (which exits immediately) then re-write.
- **Don't parse text-mode output.** It's tuned for humans and can change between versions. Always use `-o json`.
- **Don't assume newlines for missing optional fields.** JSON output omits unset optional fields entirely (`omitempty`), so `host.ups` may be absent.

---

## Environment variables `homelab-nut` reads

| Variable | When | Effect |
|---|---|---|
| `NUT_MONITOR_PASSWORD` | `apply` for `nut-client` and standalone `exporter` roles | The password `upsmon_remote` uses to read from the NUT server. Required when running these roles. |
| `EDITOR` | `inventory edit` and TUI `e` key | The editor to open; defaults to `vi`. |
| `SSH_AUTH_SOCK` | `apply` for any role | If set, ssh-agent is tried before falling back to `~/.ssh/id_ed25519`. |
| `<your-name>` (when `shutdown_daemon.slack_webhook_env` is set in the inventory) | `apply` for `shutdown-daemon` role | The actual Slack webhook URL. Not embedded in the inventory file. |

---

## Verifying your changes work

After any apply, sanity-check from outside the CLI:

```bash
# SSH into the host and check the daemon
ssh pi@192.0.2.10 systemctl is-active nut-server
ssh pi@192.0.2.10 systemctl is-active ups-battery-shutdown   # if shutdown-daemon role applied

# Or re-plan — if nothing's left to do, every host shows "no changes"
homelab-nut plan -o json | jq -r '.hosts[] | select(.diffs[].actions | length > 0)' | wc -l
# 0 = idempotent / converged
```

---

## Versioning and stability

- The contract documented here (subcommand names, JSON shapes, exit codes) is stable for the v0.x line.
- Breaking changes will bump the major (v1 → v2) per semver.
- Pre-release tags (`-alpha`, `-beta`, `-rc`) mark the release as not-yet-stable on GitHub; expect shape changes within those.
- Latest release: see [releases](https://github.com/rtorcato/homelab-nut/releases/latest).
