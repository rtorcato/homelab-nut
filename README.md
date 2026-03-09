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

### Option 2: Docker (Recommended)

```bash
cd docker

# Basic setup with web UI
docker-compose up -d

# Full stack with Prometheus + Grafana
docker-compose -f docker-compose.full-stack.yml up -d
```

Access:
- **Web UI**: http://localhost:6543
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090

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
│   ├── docker-compose.yml           # Basic NUT + Web UI
│   ├── docker-compose.full-stack.yml # Full monitoring stack
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

## License

This documentation is provided under the MIT License.

## Roadmap
If you have ideas for releases in the future, it is a good idea to list them in the README.

## Contributing
State if you are open to contributions and what your requirements are for accepting them.

For people who want to make changes to your project, it's helpful to have some documentation on how to get started. Perhaps there is a script that they should run or some environment variables that they need to set. Make these steps explicit. These instructions could also be useful to your future self.

You can also document commands to lint the code or run tests. These steps help to ensure high code quality and reduce the likelihood that the changes inadvertently break something. Having instructions for running tests is especially helpful if it requires external setup, such as starting a Selenium server for testing in a browser.

## Authors and acknowledgment
Show your appreciation to those who have contributed to the project.

## License
For open source projects, say how it is licensed.

## Project status
If you have run out of energy or time for your project, put a note at the top of the README saying that development has slowed down or stopped completely. Someone may choose to fork your project or volunteer to step in as a maintainer or owner, allowing your project to keep going. You can also make an explicit request for maintainers.
