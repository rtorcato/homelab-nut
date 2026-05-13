# services/

Daemon scripts installed and managed by `ups-service.sh`. Not intended to be run directly.

| File | Installed to | Purpose |
|---|---|---|
| `battery-shutdown.sh` | `/usr/local/bin/ups-battery-shutdown` | Polls UPS battery via `upsc` and SSHes to remote nodes to run `~/shutdown.sh` when charge drops below the configured threshold |

Managed via `sudo ../ups-service.sh`.
