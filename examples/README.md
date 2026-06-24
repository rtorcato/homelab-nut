# 📂 examples/

Reference files you copy into your own setup. **None of this runs as part of
this repo's Docker stack.**

## Inventory samples

Commented sample inventories for different topologies live in
[`inventories/`](inventories/) — start there to learn the schema by example
(minimal single-server, server + network client, SSH shutdown-targets, and a
full multi-UPS homelab). Copy whichever fits to `./homelab-nut.yaml` and edit,
or run `homelab-nut init` to generate one interactively.

## Prometheus / Grafana

Drop-in configuration snippets for scraping `nut-exporter` with Prometheus and visualising it in Grafana. See [`docs/prometheus-grafana.md`](../docs/prometheus-grafana.md) for the full how-to.

## Contents

```
examples/
├── inventories/                # Commented sample inventories, one per scenario
├── prometheus/
│   └── prometheus.yml          # Scrape job for nut-exporter — add to your prometheus.yml
└── grafana/
    ├── dashboards/
    │   └── nut-overview.json   # Importable dashboard (Grafana 10+) — 6 panels
    └── provisioning/
        ├── datasources/prometheus.yml   # For users who run Grafana via docker compose
        └── dashboards/default.yml       # File-based dashboard provider (loads /etc/grafana/provisioning/dashboards)
```

## Quick paths

- **Have a Prometheus already?** Copy the `nut` job from `prometheus/prometheus.yml` into your scrape configs and reload.
- **Have a Grafana already?** In the UI: Dashboards → New → Import → upload `grafana/dashboards/nut-overview.json` → pick your Prometheus datasource.
- **Setting up Grafana from scratch with provisioning?** Mount `grafana/provisioning/` to `/etc/grafana/provisioning/` and `grafana/dashboards/` to `/etc/grafana/provisioning/dashboards/`.

## What the dashboard expects

The dashboard queries the metrics published by [`druggeri/nut_exporter`](https://github.com/DRuggeri/nut_exporter):

| Metric | Used in |
|---|---|
| `network_ups_tools_ups_status{flag="OL\|OB\|LB\|..."}` | Status panel |
| `network_ups_tools_battery_charge` | Battery gauge, time series |
| `network_ups_tools_battery_runtime` | Runtime stat |
| `network_ups_tools_battery_voltage` | Voltages time series |
| `network_ups_tools_ups_load` | Load gauge, time series |
| `network_ups_tools_input_voltage` | Voltages time series |
| `network_ups_tools_output_voltage` | Voltages time series |

It filters by the `ups` label (multi-select) so multi-UPS setups work out of the box.
