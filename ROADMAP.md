# Roadmap — toward `homelab-nut` v1

## Where we are (2026-06-18)

**Released:** [v0.1.0-alpha](https://github.com/rtorcato/homelab-nut/releases/tag/v0.1.0-alpha) — first tagged build with cross-platform binaries via goreleaser.

**Working today:** Go CLI scaffold + Cobra subcommands (`init`, `inventory list|validate|show|edit`, `version`) + 4-screen Bubble Tea TUI + strict YAML inventory + 3 of 5 setup roles wrapping the existing bash (nut-server, nut-client, exporter).

**Mid-Phase-2:** SSH executor + role interface + 3 roles landed. Remaining for v0.1 Alpha completion: 2 more roles (shutdown-target, shutdown-daemon) and the `plan` / `apply` subcommands that wire everything into a user-visible flow.

**Live status:** [TODOS.md](TODOS.md) (auto-synced from issues) and the [Phase 2 epic #3](https://github.com/rtorcato/homelab-nut/issues/3).

## Vision

**Be the tool homelabbers reach for when they need NUT to actually do something.**

Today, getting Network UPS Tools to gracefully shut down a fleet of homelab machines is a multi-day project: install NUT on the server, install nut-client on every node, hand-edit `/etc/nut/*.conf`, generate SSH keys, write a shutdown script, install a systemd unit, configure notifications. Most people give up halfway.

`homelab-nut` will collapse that to a single command **run from your laptop**:

```bash
homelab-nut init        # interactive: discover/declare hosts and roles
homelab-nut apply       # SSH out, install + configure everything
homelab-nut status      # live UPS dashboard across the fleet
homelab-nut             # opens a full TUI for ongoing ops
```

Modeled on the tools homelabbers already love — `k9s`, `lazygit`, `lazydocker`, `gh`, `k0sctl`, `tailscale`.

---

## Architecture (committed)

- **Language:** Go (single static binary, cross-compiles to Linux/macOS/Windows ARM+x86)
- **CLI framework:** Cobra (subcommands, completions, help)
- **TUI framework:** Bubble Tea + Lipgloss + Bubbles (Charm stack — same as gh/glow/lazygit)
- **SSH:** `golang.org/x/crypto/ssh`
- **Inventory:** YAML file (`homelab-nut.yaml`) declaring hosts, roles, UPS config, shutdown rules
- **Strategy:** v1 wraps the existing tested bash scripts over SSH; v2+ progressively reimplements in native Go
- **Distribution:** Homebrew tap, `install.sh` (curl-pipe), `goreleaser` for deb/rpm/archive releases on GitHub
- **Issue tracking:** GitHub Issues + Project board + Milestones. Every TODO/FIXME in code references an issue (CI-enforced).

---

## Repo layout (target)

```
homelab-nut/
├── cmd/homelab-nut/main.go          # entry point
├── internal/
│   ├── cli/                         # cobra commands (init, apply, status, ...)
│   ├── tui/                         # bubble tea models, screens, components
│   ├── inventory/                   # YAML schema + validation
│   ├── ssh/                         # SSH executor, streaming output
│   ├── roles/                       # nut-server, nut-client, exporter, shutdown-daemon, shutdown-target
│   │   └── ...                      # each Role implements Detect/Plan/Apply
│   └── ups/                         # native NUT protocol client (Phase 6, replaces upsc shell-out)
├── scripts/                         # existing bash scripts (called by roles in v1)
├── docs/
├── examples/                        # inventory examples + Grafana dashboard
├── .github/workflows/               # ci, release, todo-check
├── go.mod
└── Makefile
```

---

## Inventory schema (sketch)

```yaml
# homelab-nut.yaml
hosts:
  - name: pi-rack
    address: 192.0.2.10
    user: pi
    roles: [nut-server, exporter, shutdown-daemon]
    ups:
      name: myups
      driver: usbhid-ups
  - name: workstation
    address: 192.0.2.20
    user: admin
    roles: [nut-client, shutdown-target]
    shutdown:
      command: ~/shutdown.sh           # path → wrapped in nohup
  - name: dream-machine
    address: 192.0.2.1
    user: admin
    roles: [shutdown-target]
    shutdown:
      command: poweroff                # inline → sent directly via SSH

shutdown_daemon:
  threshold: 50
  poll_interval: 30
  slack_webhook_env: SLACK_WEBHOOK     # read from env at apply-time, never written to disk
```

---

## Phases & milestones

### Phase 0 — Foundation (week 1)

Goal: set up the project for parallel/contributor work.

- [ ] Create GitHub Project board "homelab-nut roadmap" (kanban: Backlog / Up next / In progress / Review / Done)
- [ ] Create Milestones: `v0.1 Alpha`, `v0.2 Beta`, `v1.0 GA`
- [ ] Create labels: `area/cli`, `area/tui`, `area/ssh`, `area/roles`, `area/dist`, `good-first-issue`, `help-wanted`, `breaking`
- [ ] File epic issues (one per phase below) and child issues
- [ ] Document the TODO-must-link-to-issue policy in `CONTRIBUTING.md`
- [ ] Add `.github/workflows/todo-check.yml` — fails CI if `TODO`/`FIXME` lacks a `(#NNN)` issue reference

### Phase 1 — Skeleton (weeks 2–3) → `v0.1.0-alpha`

Goal: `homelab-nut` builds and ships a hello-world TUI + a working `inventory` command.

- [ ] `go mod init github.com/rtorcato/homelab-nut`
- [ ] Cobra root + `version` + `completion` subcommands
- [ ] `homelab-nut init` — interactive (charmbracelet/huh) prompts that write `homelab-nut.yaml`
- [ ] `homelab-nut inventory list|validate|edit` — load and validate the YAML
- [ ] Bubble Tea TUI shell: dashboard placeholder, navigation works, quit on `q`
- [ ] `Makefile`: `build`, `test`, `lint`, `run`
- [ ] CI: `go test`, `golangci-lint`, `goreleaser --snapshot` on every PR
- [ ] README hero rewrite: lead with the CLI, demo gif placeholder

### Phase 2 — SSH + first end-to-end flow (weeks 4–5) → `v0.1.0-alpha` release

Goal: `homelab-nut apply` actually sets up NUT on a real Pi via SSH.

- [ ] SSH executor: key-based auth, known_hosts handling, streaming stdout/stderr
- [ ] Role interface: `Detect() State`, `Plan(target) Diff`, `Apply(diff) error`
- [ ] Implement `nut-server` role: detect state on remote, plan diff, apply by SSHing the existing `scripts/setup-server.sh`
- [ ] `homelab-nut plan` — preview what would change per host (Terraform-style)
- [ ] `homelab-nut apply` — execute the plan, stream per-host logs
- [ ] TUI: `Apply` screen with per-host progress + collapsible log panes
- [ ] First tagged release `v0.1.0-alpha` via goreleaser

### Phase 3 — All roles + live status (weeks 6–8) → `v0.2.0-beta`

Goal: feature-complete enough to fully replace the script-based workflow.

- [ ] Roles: `nut-client`, `exporter`, `shutdown-daemon`, `shutdown-target` (each wraps a bash script for v1)
- [ ] `homelab-nut status [--watch]` — pulls UPS state via NUT protocol (port 3493), no `upsc` shell-out
- [ ] TUI Dashboard: per-host status cards, battery/load gauges, last-apply timestamp, quick actions
- [ ] `homelab-nut shutdown test` — dry-run trigger across the fleet
- [ ] `homelab-nut logs <host>` — tail `journalctl -u ups-battery-shutdown` over SSH
- [ ] Tagged release `v0.2.0-beta`

### Phase 4 — Polish + distribution (weeks 9–10) → `v1.0.0`

Goal: install in one command, look professional, work on macOS for ops from a laptop.

- [ ] Homebrew tap `rtorcato/tap` — `brew install rtorcato/tap/homelab-nut`
- [ ] `install.sh` (curl-pipe-sh) with SHA256 verification
- [ ] goreleaser: deb, rpm, archlinux, tar.gz, signed checksums
- [ ] Shell completions: bash, zsh, fish
- [ ] First-run TUI wizard: detects no inventory, walks user through `init`
- [ ] asciinema cast of full workflow → embed in README
- [ ] Demo gif on README, screenshots in docs
- [ ] Tagged release `v1.0.0` GA

### Phase 5 — Promotion (week 11+)

Goal: become the recommended NUT tool.

- [ ] r/homelab launch post with screencast
- [ ] r/selfhosted launch post
- [ ] HN Show post
- [ ] Submit PR to awesome-selfhosted
- [ ] DM the maintainers of nut-webgui, druggeri/nut_exporter — coordinate links/cross-promo

### Phase 6 — Native port (ongoing, post-v1)

Goal: drop bash dependency on remote hosts, support more platforms.

- [ ] Port `nut-server` setup to native Go (apt install + config templating + systemd unit writer)
- [ ] Same for `nut-client`, `exporter`, `shutdown-daemon`
- [ ] Native NUT protocol client (already started in Phase 3 for `status`) — extend to read all variables
- [ ] Add support for RHEL/Fedora/Alpine package managers (currently Debian/Ubuntu only)
- [ ] Optional Windows shutdown-target support (PowerShell over SSH or WinRM)
- [ ] Deprecate `scripts/` — keep one minor release with both, then remove

---

## TODOs → GitHub issues policy

**Rule:** every `TODO` or `FIXME` comment in code must reference a GitHub issue.

```go
// TODO(#42): handle multi-UPS hosts where one driver reports multiple UPS
```

```bash
# TODO(#42): handle multi-UPS hosts
```

Enforced by `.github/workflows/todo-check.yml`. Local pre-commit hook optional.

Why: comments rot, issues survive. Anyone reading the code can click through to the conversation, see who's working on it, and reproduce the discussion.

---

## Naming & branding

- **Binary:** `homelab-nut` (matches repo, googleable, branded). Aliasing as `hnut` via completion file is a possibility.
- **Module path:** `github.com/rtorcato/homelab-nut`
- **Tagline:** "Network UPS Tools, set up from your laptop."

---

## Non-goals (explicit)

- Replacing NUT itself — we stand on NUT's shoulders
- Replacing nut-webgui — it does status display well; we orchestrate setup and automation
- Enterprise / HA cluster support
- Windows / Mac NUT *server* support (clients yes, server no — NUT itself doesn't support these well)
- Building our own metrics pipeline — we emit Prometheus via the existing exporter

---

## How to contribute

This roadmap is the index. Real work happens in [GitHub Issues](https://github.com/rtorcato/homelab-nut/issues) — filter by `phase-N` label or the active milestone. See [CONTRIBUTING.md](CONTRIBUTING.md).
