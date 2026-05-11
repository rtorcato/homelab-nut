# Homelab NUT (Network UPS Tools)

This repository contains documentation and configuration examples for setting up Network UPS Tools (NUT) in a homelab environment. NUT provides a common protocol and set of tools to monitor and manage UPS (Uninterruptible Power Supply) hardware.

## Overview

NUT allows you to:
- Monitor UPS status (battery level, load, input/output voltage, etc.)
- Execute actions on power events (shutdown systems gracefully on low battery)
- Share UPS status across multiple machines on your network
- Support for 100+ different UPS manufacturers

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   UPS Device    │────▶│   NUT Server    │────▶│   NUT Client    │
│  (USB/Serial)   │     │  (nut-server)   │     │   (nut-client)  │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                              │                        │
                              │                        │
                        Monitors UPS            Receives status
                        Shares data             Triggers shutdown
```

## Documentation

| Document | Description |
|----------|-------------|
| [Server Setup](docs/server-setup.md) | How to install and configure NUT server |
| [Client Setup](docs/client-setup.md) | How to configure NUT clients to monitor remote UPS |
| [CLI Reference](docs/cli-reference.md) | NUT command-line tools and usage |
| [Docker Setup](docs/docker-setup.md) | Run NUT in Docker with web UI and monitoring |
| [Notifications](docs/notifications.md) | Slack, Discord, Pushover, Telegram alerts |
| [Smart Shutdown](docs/smart-shutdown.md) | Shutdown UniFi, LG TVs, NAS via Home Assistant |

## Quick Start

### Option 1: Automated Setup Scripts

```bash
# Server setup (Debian/Ubuntu)
sudo ./scripts/setup-server.sh myups usbhid-ups

# Client setup (provide server IP, UPS name, password from server setup)
sudo ./scripts/setup-client.sh 192.168.1.10 myups secretpassword

# Check status
./scripts/ups-status.sh myups@localhost
```

### Option 2: Docker monitoring on top of bare-metal NUT

The Docker stack is designed to run **alongside** a bare-metal `nut-server`
(it does not run its own copy of `nut-server` to avoid USB/port conflicts).
Set up bare-metal NUT first with `setup-server.sh`, then:

```bash
cd docker
cp .env.example .env
# Edit .env: set NUT_API_PASSWORD (from /root/nut-credentials.txt on the host)
# and GRAFANA_ADMIN_PASSWORD if you'll use the full stack.

# Minimal: nut-webgui + Prometheus exporter (for a remote Prometheus to scrape)
docker compose up -d

# Full stack: above + local Prometheus + Grafana (for testing)
docker compose -f docker-compose.full-stack.yml up -d
```

Access:
- **Web UI** (nut-webgui): http://localhost:9000
- **Prometheus exporter**: http://localhost:9199/ups_metrics
- **Grafana** (full stack only): http://localhost:3000
- **Prometheus** (full stack only): http://localhost:9090

### Option 3: Manual Server Setup

```bash
# Install NUT
sudo apt install nut

# Configure UPS driver, server, and users
sudo nano /etc/nut/ups.conf
sudo nano /etc/nut/upsd.conf
sudo nano /etc/nut/upsd.users

# Start services
sudo systemctl enable nut-server
sudo systemctl start nut-server
```

### Manual Client Setup (remote machines)

```bash
# Install NUT client
sudo apt install nut-client

# Configure connection to server
sudo nano /etc/nut/nut.conf
sudo nano /etc/nut/upsmon.conf

# Start monitoring
sudo systemctl enable nut-client
sudo systemctl start nut-client
```

## Supported Platforms

- Debian / Ubuntu
- RHEL / CentOS / Fedora
- Proxmox VE
- TrueNAS
- Docker

## Project Structure

```
homelab-nut/
├── README.md
├── docs/
│   ├── server-setup.md      # Server installation guide
│   ├── client-setup.md      # Client configuration guide
│   ├── cli-reference.md     # NUT CLI commands
│   ├── docker-setup.md      # Docker deployment guide
│   ├── notifications.md     # Alert configuration
│   └── smart-shutdown.md    # Device shutdown automation
├── docker/
│   ├── docker-compose.yml            # nut-webgui + Prometheus exporter
│   ├── docker-compose.full-stack.yml # + local Prometheus + Grafana
│   ├── .env.example                  # Environment template
│   ├── prometheus/
│   │   └── prometheus.yml
│   └── grafana/
│       └── provisioning/
└── scripts/
    ├── setup-server.sh      # Automated server setup
    ├── setup-client.sh      # Automated client setup
    └── ups-status.sh        # Status check script
```

## Resources

- [NUT Official Documentation](https://networkupstools.org/docs/user-manual.chunked/index.html)
- [NUT Hardware Compatibility List](https://networkupstools.org/stable-hcl.html)
- [NUT GitHub Repository](https://github.com/networkupstools/nut)
- [nut-webgui (web UI used by the Docker stack)](https://github.com/SuperioOne/nut_webgui)
- [druggeri/nut_exporter (Prometheus exporter)](https://github.com/DRuggeri/nut_exporter)

## License

MIT
