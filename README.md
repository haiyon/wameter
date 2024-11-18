# IP Monitor

IP Monitor is a versatile IP address monitoring tool that tracks both internal and external IP address changes and
provides notifications through multiple channels. It supports ESXi, Linux, macOS, and Windows platforms.

## Features

- Cross-platform support (ESXi, Linux, macOS, Windows)
- Monitors internal network interface IP changes
- Tracks external IP changes using multiple providers
- Supports multiple notification methods:
  - Email notifications (SMTP with TLS support)
  - Telegram notifications
- Configurable check intervals and retry policies
- Comprehensive logging with rotation support
- Low resource footprint

## Project Structure

```text
.
├── build-vib.sh           # VIB package build script for ESXi
├── config.example.json    # Example configuration file
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── main.go                # Application entry point
├── config/                # Configuration management
│   └── config.go          # Configuration structures and validation
├── metrics/               # Metrics collection
│   └── metrics.go         # Metrics implementation
├── monitor/               # Core monitoring logic
│   └── monitor.go         # IP monitoring implementation
├── notifier/              # Notification systems
│   ├── notifier.go        # Notification interface and manager
│   ├── email.go           # Email notification implementation
│   └── telegram.go        # Telegram notification implementation
├── scripts/                # Utility scripts
│   ├── install.sh         # Installation script for linux
│   └── build-vib.sh       # VIB package build script for ESXi
├── types/                 # Data types
│   └── types.go           # Common data types
└── utils/                 # Utility functions
    └── utils.go           # Common utilities and helpers
```

## Prerequisites

For building:

- Go 1.20 or later
- Git

Additional requirements for ESXi VIB:

- `vibauthor` (VMware VIB Author tool)

## Building

### Standard Binary

Build for your current platform:

```bash
go build -o ip-monitor
```

Cross compilation:

For Linux:

```bash
GOOS=linux GOARCH=amd64 go build -o ip-monitor-linux
```

For macOS:

```bash
GOOS=darwin GOARCH=amd64 go build -o ip-monitor-macos
```

For Windows:

```bash
GOOS=windows GOARCH=amd64 go build -o ip-monitor.exe
```

### ESXi VIB Package

```bash
./build-vib.sh
```

## Installation

### Standard Platforms (Linux/macOS/Windows)

1. Download or build the appropriate binary for your platform
2. Create configuration directory:

   ```bash
   # Linux/macOS
   sudo mkdir -p /etc/ip-monitor

   # Windows (PowerShell as Administrator)
   New-Item -Path "C:\ProgramData\ip-monitor" -ItemType Directory
   ```

3. Copy configuration:

   ```bash
   # Linux/macOS
   sudo cp config.example.json /etc/ip-monitor/config.json

   # Windows
   Copy-Item config.example.json C:\ProgramData\ip-monitor\config.json
   ```

4. Create log directory:

   ```bash
   # Linux/macOS
   sudo mkdir -p /var/log/ip-monitor

   # Windows
   New-Item -Path "C:\ProgramData\ip-monitor\logs" -ItemType Directory
   ```

### ESXi Installation

1. Copy the VIB to your ESXi host:

   ```bash
   scp dist/com.haiyon.ipm-1.0.0-1.vib root@esxi-host:/tmp/
   ```

2. Install the VIB:

   ```bash
   esxcli software vib install -v /tmp/com.haiyon.ipm-1.0.0-1.vib
   ```

## Configuration

Default configuration paths:

- Linux/macOS: `/etc/ip-monitor/config.json`
- Windows: `C:\ProgramData\ip-monitor\config.json`
- ESXi: `/etc/ipmonitor/config.json`

Example configuration:

```json
{
  "check_interval": 300,
  "network_interface": "eth0",
  "check_external_ip": true,
  "external_ip_providers": [
    "https://api.ipify.org",
    "https://ifconfig.me/ip",
    "https://icanhazip.com"
  ],
  "email": {
    "enabled": false,
    "smtp_server": "smtp.example.com",
    "smtp_port": 587,
    "username": "your-email@example.com",
    "password": "your-password",
    "from": "your-email@example.com",
    "to": [
      "recipient@example.com"
    ],
    "use_tls": true
  },
  "telegram": {
    "enabled": false,
    "bot_token": "your-bot-token",
    "chat_ids": [
      "chat-id-1",
      "chat-id-2"
    ]
  },
  "log": {
    "directory": "/var/log/ip-monitor",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true,
    "level": "info"
  }
}
```

## Running the Service

### Linux/macOS

Running directly:

```bash
./ip-monitor -config /etc/ip-monitor/config.json
```

Using systemd (Linux):

```bash
# Create service file
sudo tee /etc/systemd/system/ip-monitor.service << EOF
[Unit]
Description=IP Monitor Service
After=network.target

[Service]
ExecStart=/usr/local/bin/ip-monitor -config /etc/ip-monitor/config.json
Restart=always
User=nobody

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl enable ip-monitor
sudo systemctl start ip-monitor
```

### Windows

Running directly:

```powershell
.\ip-monitor.exe -config C:\ProgramData\ip-monitor\config.json
```

Install as a Windows Service:

```powershell
# Using NSSM (Non-Sucking Service Manager)
nssm install IPMonitor "C:\Program Files\ip-monitor\ip-monitor.exe"
nssm set IPMonitor AppParameters "-config C:\ProgramData\ip-monitor\config.json"
nssm start IPMonitor
```

### ESXi

Using service commands:

```bash
/etc/init.d/ipmonitor start
/etc/init.d/ipmonitor stop
/etc/init.d/ipmonitor status
```

## Logging

Log file locations:

- Linux/macOS: `/var/log/ip-monitor/monitor.log`
- Windows: `C:\ProgramData\ip-monitor\logs\monitor.log`
- ESXi: `/var/log/ipmonitor/monitor.log`

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
