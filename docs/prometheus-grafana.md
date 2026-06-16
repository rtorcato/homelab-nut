# Prometheus + Grafana

`homelab-nut`'s Docker stack publishes UPS metrics via [`druggeri/nut_exporter`](https://github.com/DRuggeri/nut_exporter) on port `9199`. This page shows how to scrape those metrics and visualise them in Grafana.

> **Out of scope:** running Prometheus or Grafana for you. This repo's `docker/compose.yml` only ships `nut-exporter` and `nut-webgui`. Point your existing Prometheus at the exporter; or stand up a fresh stack with the snippet at the bottom of this page.

---

## What metrics are available

Browse the raw metrics any time:

```bash
curl -s http://<host>:9199/ups_metrics?ups=myups | head -40
```

The dashboard ([`examples/grafana/dashboards/nut-overview.json`](../examples/grafana/dashboards/nut-overview.json)) uses these:

| Metric | Type | Notes |
|---|---|---|
| `network_ups_tools_ups_status{flag="OL\|OB\|LB\|CHRG\|..."}` | gauge (0/1) | One series per status flag — value is `1` when set |
| `network_ups_tools_battery_charge` | gauge | 0–100 (%) |
| `network_ups_tools_battery_runtime` | gauge | seconds remaining on battery |
| `network_ups_tools_battery_voltage` | gauge | volts |
| `network_ups_tools_ups_load` | gauge | 0–100 (%) of rated capacity |
| `network_ups_tools_input_voltage` | gauge | mains voltage |
| `network_ups_tools_output_voltage` | gauge | UPS output voltage |
| `network_ups_tools_ups_temperature` | gauge | °C (not all UPSes report this) |

Each series carries a `ups` label so multiple UPSes can be distinguished.

---

## 1. Add the scrape config to an existing Prometheus

Append this to your `prometheus.yml` (replace `nut-host` with the host running `nut-exporter`):

```yaml
scrape_configs:
- job_name: 'nut'
  scrape_interval: 15s
  static_configs:
  - targets: ['nut-host:9199']
```

Reload Prometheus (`kill -HUP $(pidof prometheus)` or `curl -X POST http://prom:9090/-/reload` if `--web.enable-lifecycle` is set).

Confirm it's working:

```bash
curl -s 'http://prom:9090/api/v1/query?query=network_ups_tools_battery_charge' | jq .
```

A snippet you can copy lives at [`examples/prometheus/prometheus.yml`](../examples/prometheus/prometheus.yml).

---

## 2. Import the dashboard into Grafana

The fastest path:

1. **Dashboards → New → Import**
2. **Upload JSON file** → pick [`examples/grafana/dashboards/nut-overview.json`](../examples/grafana/dashboards/nut-overview.json)
3. When prompted, select your **Prometheus** datasource
4. Click **Import**

What you get: a single-page view with status, battery charge, runtime, load, a voltages time-series, and a battery/load time-series. The `UPS` dropdown at the top filters by UPS name when you have more than one.

> Grafana 10+ is required (the dashboard uses schemaVersion 38). Older Grafana versions may still import but some panel options will be ignored.

---

## 3. Optional: a complete stack from scratch

If you don't have Prometheus or Grafana running anywhere yet, this minimal compose snippet brings them up alongside `nut-exporter`. **It does not replace** the project's `docker/compose.yml` — it sits next to it.

```yaml
# observability-stack.yml — drop into a separate directory
name: observability
services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    ports: ["9090:9090"]
    volumes:
      - ../homelab-nut/examples/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prom-data:/prometheus
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --storage.tsdb.retention.time=30d
      - --web.enable-lifecycle

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports: ["3000:3000"]
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=changeme
    volumes:
      - grafana-data:/var/lib/grafana
      - ../homelab-nut/examples/grafana/provisioning:/etc/grafana/provisioning:ro
      - ../homelab-nut/examples/grafana/dashboards:/etc/grafana/provisioning/dashboards:ro

volumes:
  prom-data:
  grafana-data:
```

Then:

```bash
docker compose -f observability-stack.yml up -d
# Grafana: http://localhost:3000  (admin / changeme)
# Prometheus: http://localhost:9090
```

The provisioning files in [`examples/grafana/provisioning/`](../examples/grafana/) auto-add the Prometheus datasource and load `nut-overview.json` on first boot — no UI clicks needed.

> If `nut-exporter` runs on a different host than this compose stack, edit `examples/prometheus/prometheus.yml` to use its real address (`<host>:9199`) instead of the in-network DNS name.

---

## Troubleshooting

**No data in Grafana panels.** Confirm Prometheus is scraping: open `http://prom:9090/targets` and check the `nut` job is `UP`. If down, the dashboard will be empty.

**Wrong UPS name.** The exporter takes the UPS name from a URL parameter (`?ups=myups`). Make sure your scrape config matches the UPS name configured in `/etc/nut/ups.conf`.

**Panels show "No data" but Prometheus has metrics.** The dashboard filters by the `ups` label. Some exporter versions don't add it. Check with `network_ups_tools_battery_charge` in Prometheus's Graph view — if there's no `ups=` label, update the exporter or edit the dashboard queries to drop the filter.
