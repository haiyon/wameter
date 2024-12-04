# Wameter

Wameter is a cross-platform network monitoring tool for tracking interface metrics and IP changes with multi-channel
notifications. It uses a server-agent architecture and supports multiple storage backends.

## Features

- Monitor network interfaces and traffic statistics
- Track external IP changes
- Multiple notification channels (Email, Slack, Telegram, Discord, etc.)
- Support multiple databases (SQLite, MySQL, PostgreSQL)
- RESTful API with OpenAPI documentation

## Quick Start

### Install

```bash
# From source
make build
sudo make install

# Using Docker
docker pull ghcr.io/haiyon/wameter
docker pull ghcr.io/haiyon/wameter-agent
```

### Configure

Create configuration files:

```bash
sudo mkdir -p /etc/wameter
sudo cp examples/server.example.yaml /etc/wameter/server.yaml
sudo cp examples/agent.example.yaml /etc/wameter/agent.yaml
```

Edit configurations to match your environment.

### Run

Using systemd:

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

Using Docker:

```bash
# Server
docker run -d \
  -v /etc/wameter:/etc/wameter \
  -p 8080:8080 \
  ghcr.io/haiyon/wameter

# Agent
docker run -d \
  -v /etc/wameter:/etc/wameter \
  --net=host \
  ghcr.io/haiyon/wameter-agent
```

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
