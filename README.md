# Wameter

A cross-platform network monitoring and metrics tracking tool designed to handle a wide range of network-related tasks. It utilizes a server-agent architecture, supports multiple storage backends and notification channels, and provides a RESTful API for seamless integration.

## Features

- Monitor network interfaces and traffic statistics
- Multi-channel notifications (Email, Slack, Telegram, Discord, etc.)
- Support for multiple databases (SQLite, MySQL, PostgreSQL)
- RESTful API with OpenAPI documentation
- Extensible design for future use cases

## Quick Start

### Installation

#### From Source

```bash
make build
sudo make install
```

#### Using Docker

```bash
docker pull ghcr.io/haiyon/wameter
docker pull ghcr.io/haiyon/wameter-agent
```

### Configuration

1. Create configuration files:

   ```bash
   sudo mkdir -p /etc/wameter
   sudo cp examples/server.example.yaml /etc/wameter/server.yaml
   sudo cp examples/agent.example.yaml /etc/wameter/agent.yaml
   ```

2. Edit configurations to match your environment.

### Running

#### systemd

```bash
# Server
sudo systemctl enable wameter-server
sudo systemctl start wameter-server

# Agent
sudo systemctl enable wameter-agent
sudo systemctl start wameter-agent
```

#### Docker

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

#### Command Line

```bash
wameter-server -config /etc/wameter/server.yaml
wameter-agent -config /etc/wameter/agent.yaml
```

## Updating

### From Source or Binary

For users who installed Wameter via source or binary packages, use the provided scripts to update components:

```bash
./scripts/update.sh -c [server|agent]
```

### Docker

To update Docker-based deployments, pull the latest images and recreate the containers:

```bash
# Pull the latest images
docker pull ghcr.io/haiyon/wameter
docker pull ghcr.io/haiyon/wameter-agent

# Recreate containers
docker stop <container_name>
docker rm <container_name>
docker run [options] ghcr.io/haiyon/wameter
docker run [options] ghcr.io/haiyon/wameter-agent
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
