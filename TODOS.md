# TODOs

Roadmap work tracked as GitHub Issues, grouped by phase. **Generated** — run `make todos` or `scripts/gen-todos.sh` to refresh.

> Bugs and one-off issues are tracked on GitHub directly and not listed here. This file is the index of *planned* work (issues with a `phase-*` label).

_Last updated: 2026-06-22_

## Phase 0 — Foundation ([epic #1](https://github.com/rtorcato/homelab-nut/issues/1))

Progress: 2 / 3 closed

- [ ] [#8 — Set up GitHub Project board 'homelab-nut roadmap'](https://github.com/rtorcato/homelab-nut/issues/8)
- [x] [#9 — Document TODO-to-issue policy in CONTRIBUTING.md](https://github.com/rtorcato/homelab-nut/issues/9)
- [x] [#10 — Add .github/workflows/todo-check.yml — fail CI on un-linked TODO/FIXME](https://github.com/rtorcato/homelab-nut/issues/10)

## Phase 1 — Skeleton ([epic #2](https://github.com/rtorcato/homelab-nut/issues/2)) · v0.1 Alpha

Progress: 6 / 6 closed

- [x] [#11 — homelab-nut init — interactive inventory generator (charmbracelet/huh)](https://github.com/rtorcato/homelab-nut/issues/11)
- [x] [#12 — Inventory YAML schema + validator + inventory subcommand](https://github.com/rtorcato/homelab-nut/issues/12)
- [x] [#13 — Build out the Bubble Tea TUI with multi-screen navigation](https://github.com/rtorcato/homelab-nut/issues/13)
- [x] [#14 — Rewrite README hero to lead with the CLI + add demo gif placeholder](https://github.com/rtorcato/homelab-nut/issues/14)
- [x] [#16 — Update main README to explain what homelab-nut does + introduce the CLI direction](https://github.com/rtorcato/homelab-nut/issues/16)
- [x] [#44 — feat(tui): add ASCII mascot to empty-state Dashboard](https://github.com/rtorcato/homelab-nut/issues/44)

## Phase 2 — SSH + first apply ([epic #3](https://github.com/rtorcato/homelab-nut/issues/3)) · v0.1 Alpha

Progress: 13 / 13 closed

- [x] [#17 — SSH executor — key-based auth, known_hosts, streaming output](https://github.com/rtorcato/homelab-nut/issues/17)
- [x] [#18 — Role interface — Detect/Plan/Apply over SSH](https://github.com/rtorcato/homelab-nut/issues/18)
- [x] [#19 — nut-server role — wraps scripts/setup-server.sh over SSH](https://github.com/rtorcato/homelab-nut/issues/19)
- [x] [#22 — nut-client role — wraps scripts/setup-client.sh over SSH](https://github.com/rtorcato/homelab-nut/issues/22)
- [x] [#24 — exporter role — wraps scripts/setup-exporter.sh over SSH](https://github.com/rtorcato/homelab-nut/issues/24)
- [x] [#27 — shutdown-target role — deploys ~/shutdown.sh + sudoers (script & inline modes)](https://github.com/rtorcato/homelab-nut/issues/27)
- [x] [#29 — shutdown-daemon role — installs the battery-shutdown systemd service](https://github.com/rtorcato/homelab-nut/issues/29)
- [x] [#31 — plan + apply subcommands — orchestrate SSH + roles](https://github.com/rtorcato/homelab-nut/issues/31)
- [x] [#33 — TUI Apply screen — per-host progress with collapsible log panes](https://github.com/rtorcato/homelab-nut/issues/33)
- [x] [#35 — TUI completeness — init flow + inventory edit inside the TUI](https://github.com/rtorcato/homelab-nut/issues/35)
- [x] [#36 — Machine-readable --output json + stable exit codes on subcommands](https://github.com/rtorcato/homelab-nut/issues/36)
- [x] [#37 — Two-persona README rewrite — TUI for humans, subcommands for AI/scripts](https://github.com/rtorcato/homelab-nut/issues/37)
- [x] [#38 — AGENTS.md at repo root — LLM-friendly subcommand contract](https://github.com/rtorcato/homelab-nut/issues/38)

## Phase 3 — All roles + status ([epic #4](https://github.com/rtorcato/homelab-nut/issues/4)) · v0.2 Beta

Progress: 6 / 13 closed

- [x] [#48 — feat(ups): native NUT protocol client (port 3493)](https://github.com/rtorcato/homelab-nut/issues/48)
- [x] [#49 — feat(cli): `status [--watch]` — fleet UPS state via NUT protocol](https://github.com/rtorcato/homelab-nut/issues/49)
- [x] [#50 — feat(tui): live status Dashboard with per-host cards + gauges](https://github.com/rtorcato/homelab-nut/issues/50)
- [ ] [#51 — feat(cli): `shutdown test` — dry-run shutdown trigger across the fleet](https://github.com/rtorcato/homelab-nut/issues/51)
- [ ] [#52 — feat(cli): `logs <host>` — tail journalctl over SSH](https://github.com/rtorcato/homelab-nut/issues/52)
- [ ] [#53 — feat(tui): mid-flight apply progress streaming](https://github.com/rtorcato/homelab-nut/issues/53)
- [ ] [#54 — chore(release): tag v0.2.0-beta](https://github.com/rtorcato/homelab-nut/issues/54)
- [ ] [#59 — feat(cli,tui): uninstall <host\|--all> — remove homelab-nut services + binaries from a target](https://github.com/rtorcato/homelab-nut/issues/59)
- [ ] [#60 — feat(cli,tui): backup <host> — snapshot NUT + homelab-nut config to a tarball](https://github.com/rtorcato/homelab-nut/issues/60)
- [ ] [#63 — feat(roles): per-target shutdown threshold (staged shutdown)](https://github.com/rtorcato/homelab-nut/issues/63)
- [x] [#64 — feat(tui): enrich Hosts tab — show roles, UPS state, and per-host configuration](https://github.com/rtorcato/homelab-nut/issues/64)
- [x] [#67 — fix(roles): shutdown-daemon drops per-target inline command (CMD_ override never written)](https://github.com/rtorcato/homelab-nut/issues/67)
- [x] [#68 — fix(roles): battery-shutdown daemon ignores SSH_KEY (no -i), dedicated key never used](https://github.com/rtorcato/homelab-nut/issues/68)

## Phase 4 — Polish + distribution ([epic #5](https://github.com/rtorcato/homelab-nut/issues/5)) · v1.0 GA

Progress: 1 / 1 closed

- [x] [#15 — Add Docusaurus docs site at apps/docs/ (homelab-nut.dev landing + reference)](https://github.com/rtorcato/homelab-nut/issues/15)

## Phase 5 — Promotion ([epic #6](https://github.com/rtorcato/homelab-nut/issues/6)) · v1.0 GA

_No children filed yet._

## Phase 6 — Native port ([epic #7](https://github.com/rtorcato/homelab-nut/issues/7))

_No children filed yet._

---

[Browse all issues](https://github.com/rtorcato/homelab-nut/issues) · [Bugs](https://github.com/rtorcato/homelab-nut/issues?q=is%3Aissue+is%3Aopen+label%3Abug) · [Good first issues](https://github.com/rtorcato/homelab-nut/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
