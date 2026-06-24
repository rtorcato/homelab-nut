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
homelab-nut init --no-apply                   # interactive (huh forms) — writes homelab-nut.yaml only
homelab-nut inventory validate -o json        # confirm the file parses
homelab-nut plan -o json                      # dry-run, parse to see proposed changes
homelab-nut apply --auto-approve              # execute. Add `-o json` for a machine-readable summary
```

(Without `--no-apply`, `init` applies the new inventory itself as its last step — drop the separate `apply` call.)

### "Add a host to an existing fleet"

```bash
homelab-nut inventory edit                    # opens $EDITOR with the YAML
homelab-nut inventory validate -o json        # confirm before applying
homelab-nut plan -o json | jq '.hosts[] | select(.diffs[].actions | length > 0) | .host.name'
homelab-nut apply --auto-approve -o json
```

### "What's the current state?"

```bash
homelab-nut inventory list -o json            # all hosts + their roles (declarative)
homelab-nut plan -o json                      # full diff tree (Detect ran against each host)
homelab-nut status -o json                    # live UPS state via the NUT protocol
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

Every subcommand respects the global `--inventory` / `-i` flag and the `--output` / `-o` flag (default `text`, alternative `json`). When `-i` is omitted, the inventory path resolves to `./homelab-nut.yaml` if it exists in the current directory, otherwise `~/homelab-nut.yaml` — so the tool works from any directory. `init` writes to the same resolved location.

### `homelab-nut init`

Interactive bootstrap of `homelab-nut.yaml`. **Always interactive** — uses `charmbracelet/huh` forms. Don't invoke from non-interactive contexts (no `-o json` mode); use `inventory edit` or write the YAML directly instead.

**Applies by default.** After writing the inventory, `init` immediately runs `apply` (whole fleet, auto-approved) so a fresh setup is actually live — an unapplied inventory is just inert YAML. Pass `--no-apply` to write the file only (hosts not reachable yet, or review-before-apply), then run `homelab-nut apply` later.

- **Inputs:** terminal stdin/stdout (interactive only)
- **Flags:** `--no-apply` (write the inventory but skip the apply)
- **Outputs:** writes the inventory file at the path from `--inventory`, then applies it (unless `--no-apply`)
- **Exit:** 0 on success, 1 if the user aborts or the apply reports errors

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
- **Flags:** `--auto-approve/-y`, `--concurrency N` (max parallel hosts; 0 = unlimited), `--host NAME` (apply only that host — the full inventory is still loaded so cross-host roles like nut-client resolve their dependencies; the result contains just that host).

### `homelab-nut status`

Polls each host with the `nut-server` role over the NUT TCP protocol (port 3493, native Go client — no `upsc` dependency) and reports live UPS state.

```
$ homelab-nut status -o json
[
  {
    "host": "pi-rack",
    "address": "192.0.2.10",
    "ups": "myups",
    "status": "OL",
    "battery_charge": 85,
    "battery_runtime": 1800,
    "load": 12
  },
  {
    "host": "office",
    "address": "192.0.2.11",
    "error": "timeout"
  }
]
```

- **JSON schema:** array of host objects.
  - `host`, `address` — always present.
  - `ups` — name of the first UPS reported by the server (multi-UPS hosts pick the first; future schema may switch to an array).
  - `status` — raw NUT status string (`OL`, `OB`, `OB LB`, `OL CHRG`, …). Multi-token values are kept as-is. See the status code reference below.
  - `battery_charge`, `load` — float, percent (0–100).
  - `battery_runtime` — integer, seconds.
  - `error` — non-empty when the host couldn't be reached, the UPS couldn't be listed, or a var read failed. When set, the numeric fields are omitted.
  - All numeric fields and `ups`, `status`, `error` use `omitempty` — distinguish "unknown" from a zero reading by checking presence, not value.
- **Hosts without `nut-server`** in their roles are skipped. An inventory with no nut-server hosts emits `null` / `[]`.
- **Flags:**
  - `--watch / -w` — redraw on `--interval` (default 5s) until Ctrl+C. In JSON mode, each tick emits a fresh array on its own line (NDJSON-friendly streaming consumers can read line-by-line).
  - `--timeout` — per-host TCP connect + read deadline (default 2s).
  - `-o text|json` — output format.
- **Read-only** — opens TCP connections to upsd; never SSHes anywhere.
- **Exit:** 0 in all cases; per-host failures surface in the `error` field rather than the exit code. Use `jq -e 'all(.error == null)'` if you want a fleet-wide pass/fail signal.

**Status code reference.** NUT emits short tokens for `ups.status`; multiple tokens may be combined with spaces.

| Token | Meaning | Severity |
|---|---|---|
| `OL` | On Line — utility power, healthy | ok |
| `OL CHRG` | On Line, battery is charging | ok |
| `OB` | On Battery — utility power out, UPS sustaining load | warn |
| `OB LB` | On Battery + Low Battery — imminent shutdown | critical |
| `LB` | Low Battery (regardless of OL/OB) | critical |
| `RB` | Replace Battery | warn |
| `CHRG` / `DISCHRG` | Charging / Discharging modifier | informational |
| `BYPASS` | Bypass active (load on raw mains) | warn |
| `CAL` | Calibration in progress | informational |
| `OVER` / `TRIM` / `BOOST` | Overload / voltage trim / voltage boost | warn |
| `FSD` | Forced Shutdown initiated | critical |
| `OFF` | UPS reports itself offline | warn |

The TUI Dashboard color-codes badges using OL → green, OB → amber, anything with `LB` → red, errors → red `ERR`, empty/unknown → grey `?`. Scripts consuming `-o json` should parse `status` themselves and apply whatever rules they need — the JSON contract just carries the raw NUT string.

### `homelab-nut shutdown test`

Dry-runs the shutdown chain across the fleet **without powering anything off**. For each host with the `shutdown-target` role, it SSHes in as the configured user and verifies the host's shutdown command can be resolved:

- a script path (first token contains `/`, e.g. `~/shutdown.sh`) must exist and be executable (`test -x`; `~/` is expanded via `$HOME` to mirror how the daemon runs it);
- an inline command (e.g. `poweroff`, or the first token of `shutdown -h now`) must be found in `PATH` (`command -v`).

```
$ homelab-nut shutdown test -o json
[
  { "host": "workstation",   "command": "~/shutdown.sh", "ok": true },
  { "host": "dream-machine", "command": "poweroff",      "ok": false, "error": "command not found in PATH: poweroff" }
]
```

- **JSON schema:** array of `{ host, command, ok, error }`.
  - `host` — inventory host name.
  - `command` — the resolved shutdown command (empty if the host has none configured).
  - `ok` — `true` if the command is runnable on that host.
  - `error` — `omitempty`; the specific failure (`no shutdown command configured`, an SSH error, `command not found in PATH: …`, or `script not found or not executable: …`).
- **Hosts without `shutdown-target`** in their roles are skipped. An inventory with no shutdown-target hosts prints a notice (text) / `[]` (json) and exits 0.
- **Never powers anything off** — it only runs `test -x` / `command -v`.
- **Flags:** `-o text|json`.
- **Exit:** 0 if every checked host passes; `3` if any host fails the check (per-host detail is in each result's `error`).

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
    shutdown_daemon:             # per-host daemon tuning (host with `shutdown-daemon` role)
      threshold: 50              # 1-99 (% battery)
      poll_interval: 30          # seconds, > 0
      slack_webhook_env: SLACK_WEBHOOK   # optional; env var name (not URL) at apply time
  - name: workstation
    address: 192.0.2.20
    user: admin
    roles: [nut-client, shutdown-target]
    shutdown:                    # used when host has role `shutdown-target`
      command: ~/shutdown.sh     # path → deployed; bare cmd (e.g. `poweroff`) → sent inline
      delay: 0                   # optional; seconds the daemon waits before sending this
                                 # target's shutdown (sequence dependents, e.g. gateway last)
shutdown_daemon:                 # OPTIONAL fleet-wide default; a per-host block overrides it
  threshold: 50                  # 1-99 (% battery)
  poll_interval: 30              # seconds, > 0
  slack_webhook_env: SLACK_WEBHOOK   # optional; env var name (not URL) at apply time
```

**Role enum:** `nut-server`, `nut-client`, `exporter`, `shutdown-daemon`, `shutdown-target`.

**Shutdown-daemon config precedence:** a host's own `shutdown_daemon` block →
the root-level `shutdown_daemon` block (fleet-wide default) → built-in 50% / 30s.
The root block is optional and kept for back-compat; new inventories put the
config on the daemon host. Both `host.shutdown_daemon` and root `shutdown_daemon`
appear in `inventory show -o json`.

**Validation rules** (enforced by `inventory validate`, also at `plan` and `apply` time):
- Required: `hosts[].name`, `address`, `user`, `roles`
- No duplicate host names
- `nut-server` requires `ups.name` + `ups.driver`
- `shutdown-target` with a `shutdown` block needs `command`; `shutdown.delay` (if set) must be ≥ 0 (seconds)
- a root `shutdown_daemon` block requires at least one host with `shutdown-daemon` role
- a per-host `shutdown_daemon` block requires that host to have the `shutdown-daemon` role
- `threshold` ∈ [1, 99], `poll_interval` > 0 (both the global block and per-host overrides)

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
| `NUT_MONITOR_PASSWORD` | `apply` for `nut-client` and standalone `exporter` roles | The password `upsmon_remote` uses to read from the NUT server. **Optional** — if unset, `apply` auto-discovers it by SSHing into the resolved `nut-server` host and reading `/root/nut-credentials.txt` (requires the server to have been applied already). Set this to override or for CI where SSH-to-server isn't available. |
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
