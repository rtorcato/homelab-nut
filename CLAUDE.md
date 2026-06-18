# CLAUDE.md — developing on `homelab-nut`

Repo-specific context for AI agents (Claude Code, Cursor, Aider, Codex, …)
**editing the codebase**. If you're an AI agent *using* `homelab-nut`
as a tool (parsing `apply -o json`, reading exit codes, etc.), see
**[AGENTS.md](AGENTS.md)** instead — different audience.

---

## What this project is

A Go CLI + Bubble Tea TUI for setting up [Network UPS Tools (NUT)](https://networkupstools.org/) across a homelab fleet via SSH. Roles wrap battle-tested bash scripts that ship in `/scripts/` (wrap-then-port strategy — Phase 6 of the ROADMAP ports them to native Go).

**Two equally first-class paths in** (the design principle that should anchor every UX decision):
- **Humans** → run `homelab-nut`, use the TUI. Should never need to remember a subcommand.
- **AI agents / scripts** → use the subcommands with `-o json`. Stable JSON shapes, stable exit codes, documented in AGENTS.md.

Don't demote either path. If you add a TUI screen, expose the same capability as a subcommand. If you add a subcommand, make sure the TUI can reach the same behaviour.

---

## Build, test, lint

| Command | Purpose |
|---|---|
| `make build` | Build `bin/homelab-nut` with ldflags-injected version |
| `make test` | `go test -race -count=1 ./...` |
| `make lint` | `golangci-lint run` (CI installs from source — see `.github/workflows/go-ci.yml`) |
| `make tidy` | `go mod tidy` |
| `make run` | Build and launch the TUI |
| `make snapshot` | Cross-compile via goreleaser (snapshot mode, no publish) |
| `make embed-sync` | Copy `/scripts/*.sh` into `internal/roles/embedded/` — **required after editing any wrapped script** |
| `make todos` | Regenerate `TODOS.md` from GitHub issues |
| `make docs-cli` | Regenerate Docusaurus CLI reference from cobra |
| `make docs-dev` / `make docs-build` | Local docs site dev server / static build |

**Go version:** 1.25+ (driven by x/crypto v0.53). CI uses 1.25.
**Local dev:** `brew install go` works fine.

---

## Working in this repo — the PR workflow

`main` is **branch-protected**. The only way changes land is through a PR with all required status checks green. Direct pushes are rejected.

The full workflow lives in the **`github-pr-workflow` skill** (stowed at `~/.claude/skills/github-pr-workflow/SKILL.md` in the maintainer's dotfiles, also covered in [CONTRIBUTING.md](CONTRIBUTING.md)). Short version:

```bash
git fetch origin main
git switch -c <type>/<issue>-<short-name> origin/main   # e.g. feat/42-add-thing
# ... work ...
git commit -m "feat(area): conventional-commit subject"
git push -u origin <branch>
gh pr create --fill --title "feat(area): conventional-commit subject"
gh pr merge --auto --squash --delete-branch
```

**Branch naming:** `feat/N-name`, `fix/N-name`, `docs/N-name`, `chore/N-name`, `ci/N-name`, `refactor/N-name`, `test/N-name` where N is the GitHub issue number.

**Required CI checks** (all must pass before merge):
- `build + test`
- `golangci-lint`
- `goreleaser snapshot`
- `shellcheck`
- `docker compose validate`
- `TODO/FIXME must reference an issue`

**PR title format:** Conventional Commit (`feat(roles): add nut-client role`). Becomes the squash commit subject on `main`.

**Don't put the issue number in the PR title.** GitHub appends the PR number automatically when squash-merging; doubling them up gives `(#22) (#23)` noise.

---

## Code layout

```
cmd/
├── homelab-nut/main.go      # binary entry point — ldflags-injected version
└── gen-docs/main.go         # cobra→markdown for the docs site (make docs-cli)

internal/
├── cli/                     # cobra command tree
│   ├── root.go              # root cmd, TUI dispatcher loop (handles 'i' / 'e' / quit)
│   ├── inventory.go         # `inventory list/show/validate/edit`
│   ├── init.go              # `init` (thin wrapper over forms package)
│   ├── plan.go              # `plan`
│   ├── apply.go             # `apply` (with --auto-approve, --concurrency, -o json)
│   ├── version.go
│   ├── output.go            # --output flag + emitJSON helper
│   └── exit.go              # named exit codes — STABLE CONTRACT (see AGENTS.md)
├── tui/                     # Bubble Tea screens
│   ├── model.go             # root model, screen state, key handlers, ExitAction()
│   ├── apply.go             # Apply screen
│   └── styles.go            # lipgloss palette tuned to cover.png brand
├── forms/                   # shared huh form definitions (used by both CLI init + TUI 'i')
├── inventory/               # YAML schema, loader, validator, save
├── roles/                   # Role interface + 5 concrete roles
│   └── embedded/            # mirror of /scripts/*.sh — see embed-sync rule below
├── orchestrator/            # per-host concurrency, role ordering, Plan/Apply
└── ssh/                     # connection executor, key auth, known_hosts

scripts/                     # canonical bash scripts (the "wrap" side of wrap-then-port)
├── setup-server.sh / setup-client.sh / setup-exporter.sh / setup-shutdown-daemon.sh
├── services/battery-shutdown.sh
└── remote-host/{deploy,shutdown}.sh

apps/docs/                   # Docusaurus site (deployed to GitHub Pages)
examples/                    # homelab-nut.yaml + Grafana dashboard
docker/                      # Docker compose (nut-exporter + nut-webgui)
.github/workflows/           # CI: Go CI, shellcheck CI, TODO check, docs deploy, release, TODOs sync
```

---

## The embed-sync rule (don't break this)

`internal/roles/embedded/*.sh` are **byte-for-byte mirrors** of `/scripts/*.sh`. Required because Go's `//go:embed` can't reach out of the package directory with `..`.

**If you edit any script in `/scripts/`, run `make embed-sync` before committing.** CI's `embed-sync drift check` (in `.github/workflows/go-ci.yml`) fails the build if the mirror diverges.

---

## TODOs link to issues (CI-enforced)

Every `TODO` or `FIXME` in source code must reference a GitHub issue:

```go
// TODO(#42): handle multi-UPS hosts
```

CI (`.github/workflows/todo-check.yml`) fails on any bare `TODO`/`FIXME` without a `(#NNN)` reference. The regex requires a comment marker (`//`, `#`, `/*`) before the token, so `context.TODO()` won't false-trigger.

For temporary notes that aren't deferred work, use `NOTE` or `XXX` instead — they're not policed.

Full policy in [CONTRIBUTING.md](CONTRIBUTING.md#todos-link-to-issues).

---

## Issue conventions

- **Labels:** `area/cli`, `area/tui`, `area/ssh`, `area/roles`, `area/dist`, `area/docs`, `phase-0` … `phase-6`, `breaking`
- **Milestones:** `v0.1 Alpha`, `v0.2 Beta`, `v1.0 GA`
- **Phase epics:** issues #1 – #7 are the phase epics from the ROADMAP. New work files as a child issue with the matching `phase-N` label.

To file a new issue from the CLI:

```bash
gh issue create --title "..." --label "phase-N,area/..." --milestone "..." --body "..."
```

---

## Where the answers are

| Question | File |
|---|---|
| What's the project's vision and phasing? | [ROADMAP.md](ROADMAP.md) |
| What's open / in-flight right now? | [TODOS.md](TODOS.md) (auto-synced from issues) |
| How do contributors propose changes? | [CONTRIBUTING.md](CONTRIBUTING.md) |
| How do AI agents *use* the tool? | [AGENTS.md](AGENTS.md) |
| Full CLI reference with subcommand schemas | [Docs site](https://rtorcato.github.io/homelab-nut/docs/cli) |
| Skill: GitHub PR workflow conventions | `~/.claude/skills/github-pr-workflow/SKILL.md` (in dotfiles) |

---

## Gotchas

- **Don't push direct to `main`** — branch protection rejects it. Always go through a PR.
- **`gh pr merge --auto`** before branch protection existed would merge immediately. With protection live now, auto-merge correctly *queues* and only fires when required checks go green.
- **`tea.ExecProcess` doesn't fit huh forms** — the TUI 'i' / 'e' keys instead use the `exitAction` field on `rootModel`. Root cmd checks `tui.ExitAction(finalModel)` after the TUI program returns and dispatches accordingly, then relaunches the TUI in a loop. See `internal/cli/root.go:runTUILoop`.
- **Inventory has to ride along on `context.Context`** for cross-host roles (`nut-client`, `exporter` standalone, `shutdown-daemon`) to resolve their dependencies. Use `roles.WithInventory(ctx, inv)` when building a context, `roles.inventoryFrom(ctx)` to read it.
- **`apply -o json` requires `--auto-approve`** — refuses to start without it (would hang on the y/N prompt with no TTY). Documented in AGENTS.md.
- **`State.MarshalJSON` emits string labels** (`"ok"`, `"missing"`) not integer enum values. Don't bypass it by writing a custom encoder; downstream consumers (AGENTS.md schema) depend on the string form.
- **JSON tags on inventory structs sit alongside YAML tags** (e.g. ``yaml:"name" json:"name"``). When adding new fields, add both.
- **`go vet` runs in CI** and is non-negotiable — it catches things like discarded `context.WithTimeout` cancel functions and `io.WriterTo` signature mismatches.
- **golangci-lint config in `.golangci.yml`** exempts the conventional safe-to-ignore errcheck patterns (`fmt.Fprint*`, `(*os.File).Close`, `ssh.Session.Close`). Don't add new exemptions without confirming the pattern is genuinely always-safe.
