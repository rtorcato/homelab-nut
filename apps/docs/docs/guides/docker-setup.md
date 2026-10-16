# NUT Docker Setup

Run NUT server and monitoring tools in Docker containers.

## Table of Contents

- [Quick Start](#quick-start)
- [Overview](#overview)
- [NUT Server Container](#nut-server-container)
- [NUT Web UI](#nut-web-ui)
- [Prometheus Exporter](#prometheus-exporter)
- [Full Stack with Grafana](#full-stack-with-grafana)
- [Docker Compose Examples](#docker-compose-examples)

## Quick Start

```bash
cd docker

# Copy and configure environment
cp .env.example .env
nano .env  # Set your passwords and UPS config

# Start basic stack (NUT + Web UI + Exporter)
docker compose up -d

# Or start full monitoring stack with Grafana
docker compose -f docker-compose.full-stack.yml up -d
```

Access:
- **Web UI**: http://localhost:6543
- **Prometheus Metrics**: http://localhost:9199/metrics
- **Grafana** (full-stack): http://localhost:3000

## Overview

Running NUT in Docker provides:
- Easy deployment and updates
- Isolated configuration
- Portable across systems
- Simple integration with monitoring stacks

## NUT Server Container

### Using upsd/nut-upsd

The most popular NUT server image:

```yaml
# docker-compose.yml

services:
  nut-server:
    image: instantlinux/nut-upsd:latest
    container_name: nut-server
    hostname: nut-server
    restart: unless-stopped
    ports:
      - "3493:3493"
    environment:
      - API_USER=upsmon
      - API_PASSWORD=your_secure_password
      - DESCRIPTION=APC Back-UPS 1500
      - DRIVER=usbhid-ups
      - GROUP=nut
      - NAME=myups
      - POLLINTERVAL=
      - PORT=auto
      - SDORDER=
      - SECRET=your_admin_secret
      - SERIAL=
      - SERVER=master
      - USER=nut
      - VENDORID=
    devices:
      - /dev/bus/usb:/dev/bus/usb
    privileged: true
```

### Using networkupstools/nut-upsd

Official NUT project image:

```yaml
version: '3.8'

services:
  nut-server:
    image: networkupstools/nut-upsd:latest
    container_name: nut-upsd
    restart: unless-stopped
    ports:
      - "3493:3493"
    volumes:
      - ./config/ups.conf:/etc/nut/ups.conf:ro
      - ./config/upsd.conf:/etc/nut/upsd.conf:ro
      - ./config/upsd.users:/etc/nut/upsd.users:ro
      - ./config/nut.conf:/etc/nut/nut.conf:ro
    devices:
      - /dev/bus/usb:/dev/bus/usb
    privileged: true
```

Create config files:

```bash
mkdir -p config

# config/nut.conf
cat > config/nut.conf << 'EOF'
MODE=netserver
EOF

# config/ups.conf
cat > config/ups.conf << 'EOF'
[myups]
    driver = usbhid-ups
    port = auto
    desc = "APC Back-UPS 1500"
EOF

# config/upsd.conf
cat > config/upsd.conf << 'EOF'
LISTEN 0.0.0.0 3493
EOF

# config/upsd.users
cat > config/upsd.users << 'EOF'
[admin]
    password = your_admin_password
    actions = SET
    instcmds = ALL

[upsmon]
    password = your_monitor_password
    upsmon master

[upsmon_remote]
    password = your_remote_password
    upsmon slave
EOF

chmod 600 config/upsd.users
```

## NUT Web UI

### webNUT

Simple Python-based web interface:

```yaml
version: '3.8'

services:
  webnut:
    image: teknologist/webnut:latest
    container_name: webnut
    restart: unless-stopped
    ports:
      - "6543:6543"
    environment:
      - UPS_HOST=nut-server
      - UPS_PORT=3493
    depends_on:
      - nut-server
```

Access at `http://your-server:6543`

### NUT Dashboard (Node.js)

Modern dashboard interface:

```yaml
version: '3.8'

services:
  nut-dashboard:
    image: brandawg93/nut_dashboard:latest
    container_name: nut-dashboard
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - NUT_HOST=nut-server
      - NUT_PORT=3493
      - WEB_PORT=3000
```

### UPS Admin (PHP)

```yaml
version: '3.8'

services:
  upsadmin:
    image: halfs13/nut-web:latest
    container_name: nut-web
    restart: unless-stopped
    ports:
      - "8080:80"
    environment:
      - NUT_HOST=nut-server
      - NUT_PORT=3493
```

## Prometheus Exporter

Export NUT metrics to Prometheus for alerting and visualization.

### NUT Exporter

```yaml
version: '3.8'

services:
  nut-exporter:
    image: druggeri/nut_exporter:latest
    container_name: nut-exporter
    restart: unless-stopped
    ports:
      - "9199:9199"
    environment:
      - NUT_EXPORTER_SERVER=nut-server
      - NUT_EXPORTER_USERNAME=upsmon
      - NUT_EXPORTER_PASSWORD=your_monitor_password
    command:
      - '--nut.server=nut-server'
      - '--nut.username=upsmon'
      - '--nut.password=your_monitor_password'
```

### Prometheus Configuration

Add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'nut'
    static_configs:
      - targets: ['nut-exporter:9199']
    scrape_interval: 30s
```

### Available Metrics

| Metric | Description |
|--------|-------------|
| `nut_battery_charge` | Battery charge percentage |
| `nut_battery_runtime_seconds` | Estimated runtime |
| `nut_battery_voltage` | Battery voltage |
| `nut_input_voltage` | Input voltage |
| `nut_output_voltage` | Output voltage |
| `nut_ups_load` | UPS load percentage |
| `nut_ups_status` | UPS status flags |

## Full Stack with Grafana

Complete monitoring stack with Prometheus and Grafana:

```yaml
version: '3.8'

services:
  nut-server:
    image: instantlinux/nut-upsd:latest
    container_name: nut-server
    restart: unless-stopped
    ports:
      - "3493:3493"
    environment:
      - API_USER=upsmon
      - API_PASSWORD=secret123
      - DRIVER=usbhid-ups
      - NAME=myups
      - PORT=auto
      - SERVER=master
    devices:
      - /dev/bus/usb:/dev/bus/usb
    privileged: true

  nut-exporter:
    image: druggeri/nut_exporter:latest
    container_name: nut-exporter
    restart: unless-stopped
    ports:
      - "9199:9199"
    command:
      - '--nut.server=nut-server'
    depends_on:
      - nut-server

  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    depends_on:
      - nut-exporter

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    depends_on:
      - prometheus

  webnut:
    image: teknologist/webnut:latest
    container_name: webnut
    restart: unless-stopped
    ports:
      - "6543:6543"
    environment:
      - UPS_HOST=nut-server
      - UPS_PORT=3493
    depends_on:
      - nut-server

volumes:
  prometheus_data:
  grafana_data:
```

Create Prometheus config:

```bash
mkdir -p prometheus

cat > prometheus/prometheus.yml << 'EOF'
global:
  scrape_interval: 30s
  evaluation_interval: 30s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'nut'
    static_configs:
      - targets: ['nut-exporter:9199']
EOF
```

Create Grafana provisioning:

```bash
mkdir -p grafana/provisioning/datasources
mkdir -p grafana/provisioning/dashboards

cat > grafana/provisioning/datasources/prometheus.yml << 'EOF'
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
EOF
```

## Docker Compose Examples

### Minimal Server Only

```yaml
# docker-compose.server.yml
version: '3.8'

services:
  nut-server:
    image: instantlinux/nut-upsd:latest
    container_name: nut-server
    restart: unless-stopped
    ports:
      - "3493:3493"
    environment:
      - API_USER=upsmon
      - API_PASSWORD=changeme
      - DRIVER=usbhid-ups
      - NAME=myups
      - PORT=auto
      - SERVER=master
    devices:
      - /dev/bus/usb:/dev/bus/usb
    privileged: true
```

### Server + Web UI

```yaml
# docker-compose.web.yml
version: '3.8'

services:
  nut-server:
    image: instantlinux/nut-upsd:latest
    container_name: nut-server
    restart: unless-stopped
    ports:
      - "3493:3493"
    environment:
      - API_USER=upsmon
      - API_PASSWORD=changeme
      - DRIVER=usbhid-ups
      - NAME=myups
      - PORT=auto
      - SERVER=master
    devices:
      - /dev/bus/usb:/dev/bus/usb
    privileged: true

  webnut:
    image: teknologist/webnut:latest
    container_name: webnut
    restart: unless-stopped
    ports:
      - "6543:6543"
    environment:
      - UPS_HOST=nut-server
      - UPS_PORT=3493
    depends_on:
      - nut-server
```

### Client Only (Remote Monitoring)

```yaml
# docker-compose.client.yml
version: '3.8'

services:
  nut-exporter:
    image: druggeri/nut_exporter:latest
    container_name: nut-exporter
    restart: unless-stopped
    ports:
      - "9199:9199"
    command:
      - '--nut.server=192.168.1.10'  # Your NUT server IP
      - '--nut.username=upsmon'
      - '--nut.password=your_password'
```

## Deployment

### Start Services

```bash
# Start all services
docker-compose up -d

# Start specific compose file
docker-compose -f docker-compose.web.yml up -d

# View logs
docker-compose logs -f nut-server

# Check status
docker-compose ps
```

### Verify UPS Connection

```bash
# Check if UPS is detected
docker exec nut-server upsc myups

# Check USB devices inside container
docker exec nut-server lsusb
```

### Troubleshooting

```bash
# View container logs
docker logs nut-server

# Shell into container
docker exec -it nut-server /bin/sh

# Check driver
docker exec nut-server upsdrvctl status

# Test from host
upsc myups@localhost
```

## USB Passthrough Notes

### Linux Host

USB passthrough works with `devices` and `privileged`:

```yaml
devices:
  - /dev/bus/usb:/dev/bus/usb
privileged: true
```

Or more restrictive (find your UPS device):

```bash
# Find UPS device
lsusb
# Example: Bus 001 Device 003

# Use specific device
devices:
  - /dev/bus/usb/001/003:/dev/bus/usb/001/003
```

### Proxmox LXC

For LXC containers, add to container config:

```
lxc.cgroup.devices.allow: c 189:* rwm
lxc.mount.entry: /dev/bus/usb dev/bus/usb none bind,optional,create=dir
```

### Docker on Proxmox VM

Passthrough USB to VM first, then to Docker:

1. In Proxmox: Add USB device to VM
2. In VM: Configure Docker with USB device

## Security Considerations

1. **Don't use `privileged: true` in production** if possible - use specific device mounts
2. **Change default passwords** in all examples
3. **Use Docker secrets** for sensitive data:

```yaml
services:
  nut-server:
    secrets:
      - nut_password
    environment:
      - API_PASSWORD_FILE=/run/secrets/nut_password

secrets:
  nut_password:
    file: ./secrets/nut_password.txt
```

4. **Limit network exposure** - bind to internal networks only if not needed externally
