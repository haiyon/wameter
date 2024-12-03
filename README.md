# Wameter

Wameter is a cross-platform network monitoring tool for tracking interface metrics and IP changes with multi-channel
notifications. It uses a server-agent architecture and supports multiple storage backends.

## Features

- Network interface monitoring (physical and virtual)
- Real-time network statistics and external IP tracking
- Multiple notification channels (Email, Telegram, Webhook, Slack)
- Flexible storage backends (SQLite, MySQL, PostgreSQL)
- Cross-platform support (Linux, macOS)
- RESTful API

## Quick Start

### Installation

Using installation script:

```bash
# Install server
./scripts/install.sh -c server

# Install agent
./scripts/install.sh -c agent
```

### Configuration

Server configuration (`/etc/wameter/server.yaml`):

```yaml
server:
  address: ":8080"

storage:
  driver: "sqlite"
  dsn: "/var/lib/wameter/data.db"

notify:
  email:
    enabled: true
    smtp_server: "smtp.example.com"
  telegram:
    enabled: true
    bot_token: "your-bot-token"
```

Agent configuration (`/etc/wameter/agent.yaml`):

```yaml
agent:
  id: "agent-1"
  server:
    address: "http://localhost:8080"

collector:
  network:
    enabled: true
    interfaces: ["eth0"]
    check_external_ip: true
```

### Running

Using systemd (Linux):

```bash
# Server
sudo systemctl enable wameter-server
sudo systemctl start wameter-server

# Agent
sudo systemctl enable wameter-agent
sudo systemctl start wameter-agent
```

Using command line:

```bash
wameter-server -config /etc/wameter/server.yaml
wameter-agent -config /etc/wameter/agent.yaml
```

## API Endpoints

- `GET /api/v1/metrics` - Get monitoring metrics
- `GET /api/v1/agents` - List agents
- `GET /api/v1/agents/:id` - Get agent details
- `POST /api/v1/agents/:id/command` - Send command to agent

Full API documentation is available at the `/docs` endpoint.

## Maintenance

Update components:

```bash
./scripts/update.sh -c [server|agent]
```

Uninstall components:

```bash
./scripts/uninstall.sh -c [server|agent]
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
