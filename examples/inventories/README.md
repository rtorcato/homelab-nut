# 📂 examples/inventories/

Commented sample inventories — one per scenario, from simplest to most complete.
Copy one to `./homelab-nut.yaml`, edit the addresses/users, and run
`homelab-nut inventory validate` (or `homelab-nut apply`).

Every file here is validated in CI (`internal/inventory/examples_test.go`), so
the samples can't drift out of sync with the schema.

| File | Scenario | Roles shown |
|---|---|---|
| [`minimal.yaml`](minimal.yaml) | One host that owns the UPS and shuts itself down. | `nut-server` |
| [`server-and-client.yaml`](server-and-client.yaml) | One UPS, a server that owns it + another machine that shuts down over the network. | `nut-server`, `nut-client` |
| [`ssh-shutdown-targets.yaml`](ssh-shutdown-targets.yaml) | Daemon SSHes shutdowns into a NAS and a UniFi gateway; gateway powered off last via `delay`. | `nut-server`, `exporter`, `shutdown-daemon`, `shutdown-target` |
| [`full-homelab.yaml`](full-homelab.yaml) | Two UPSes, per-host daemon thresholds, a client, SSH targets, Prometheus exporter. | all five |

Validate any of them:

```bash
homelab-nut inventory validate -i examples/inventories/minimal.yaml
```

The schema and per-field rules are documented in [AGENTS.md](../../AGENTS.md#inventory-schema)
and the [docs site](https://rtorcato.github.io/homelab-nut/docs/intro).
