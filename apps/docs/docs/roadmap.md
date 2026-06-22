---
sidebar_position: 3
title: Roadmap
---

# Roadmap

The canonical roadmap lives at [ROADMAP.md](https://github.com/rtorcato/homelab-nut/blob/main/ROADMAP.md) in the repo. Live status of open work is at [TODOS.md](https://github.com/rtorcato/homelab-nut/blob/main/TODOS.md) (regenerated from GitHub Issues with `make todos`).

This page is a thin summary. Read those for the full plan.

## Vision

> Be the tool homelabbers reach for when they need NUT to actually do something.

Today, getting NUT to coordinate graceful shutdown across more than one machine is a multi-day project. `homelab-nut` collapses that to a single command **run from your laptop**:

```bash
homelab-nut init        # interactive: discover/declare hosts and roles
homelab-nut apply       # SSH out, install + configure everything
homelab-nut status      # live UPS dashboard across the fleet
homelab-nut             # opens a full TUI for ongoing ops
```

Modeled on the tools homelabbers already love — `k9s`, `lazygit`, `lazydocker`, `gh`, `k0sctl`, `tailscale`.

## Phases

| Phase | What | Milestone |
|---|---|---|
| **0** — Foundation | Issues, labels, project board, CI policy | (housekeeping) |
| **1** — Skeleton | Go module, Cobra, Bubble Tea TUI, inventory schema, init wizard | **v0.1 Alpha** ✅ |
| **2** — SSH + first apply | SSH executor, `nut-server` role end-to-end | v0.1 Alpha |
| **3** — All roles + status | Every role wrapped, live status dashboard | v0.2 Beta |
| **4** — Polish + distribution | Homebrew tap, install.sh, asciinema, this site | **v1.0 GA** |
| **5** — Promotion | r/homelab launch, awesome-selfhosted, HN | v1.0 GA |
| **6** — Native port | Drop bash dependency, multi-distro support | post-v1 |

## Architecture (committed)

- **Language:** Go (single static binary, cross-compiles to Linux/macOS/Windows ARM+x86)
- **CLI framework:** Cobra
- **TUI framework:** Bubble Tea + Lipgloss (Charm)
- **SSH:** `golang.org/x/crypto/ssh`
- **Inventory:** YAML `homelab-nut.yaml`
- **Strategy:** wrap existing bash scripts via SSH in v1; progressively port to native Go in Phase 6
- **Distribution:** Homebrew tap, `install.sh`, goreleaser (deb/rpm/tar.gz)

## Non-goals

- Replacing NUT itself — we stand on NUT's shoulders
- Replacing nut-webgui — it does status display well; we orchestrate setup and automation
- Enterprise / HA cluster support
- Windows / Mac NUT *server* support (clients yes, server no)
- Building our own metrics pipeline — we emit Prometheus via the existing exporter
