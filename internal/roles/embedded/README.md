# Embedded scripts

These bash scripts are bundled into the `homelab-nut` binary via
`embed.FS` (see `../embedded.go`). The roles package pipes them through
SSH so a fresh remote host doesn't need anything pre-installed beyond
`bash` and `sudo`.

**They are byte-for-byte mirrors of the canonical scripts under `/scripts/`.**
This duplication exists because Go's `//go:embed` directive only sees
files inside the package directory tree — it cannot reach up into
`/scripts/`.

## Keeping them in sync

After editing any script in `/scripts/`, run:

```
make embed-sync
```

CI (`.github/workflows/go-ci.yml`) runs the sync and fails if it
produces a diff, so drift can't land on `main`.

## Files

| Embedded | Canonical | Role |
|---|---|---|
| `setup-server.sh` | `/scripts/setup-server.sh` | `roles.nutServer` |
