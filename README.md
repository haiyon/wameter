# Wameter

**Wameter** is a versatile IP address monitoring tool that tracks both internal and external IP address changes and
provides notifications through multiple channels. It supports ESXi, Linux, macOS, and Windows platforms.

## Features

- Cross-platform support (ESXi, Linux, macOS, Windows)
- Comprehensive network interface monitoring:
  - Monitors multiple network interfaces simultaneously
  - Supports various interface types (ethernet, wireless, bridge, etc.)
  - Optional monitoring of virtual interfaces
  - Customizable interface filtering
  - Network interface statistics collection
- Tracks external IP changes using multiple providers
- Detailed interface statistics:
  - Throughput monitoring (rx/tx bytes)
  - Packet statistics
  - Error tracking
  - Interface status monitoring
- Supports multiple notification methods:
  - Email notifications (SMTP with TLS support)
  - Telegram notifications
- Advanced notification features:
  - Per-interface change notifications
  - Network statistics reporting
  - Customizable notification content
  - Smart message formatting
- Configurable check intervals and retry policies
- Comprehensive logging with rotation support
- Low resource footprint

## Interface Monitoring Features

- **Interface Type Support**:

  - Physical interfaces (ethernet, wireless)
  - Virtual interfaces (optional)
  - Bridge interfaces
  - VLAN interfaces
  - Bonding/Team interfaces
  - Tunnel interfaces

- **Interface Statistics**:

  - Bandwidth utilization
  - Packet counts
  - Error statistics
  - Interface status
  - MTU information
  - MAC addresses

- **Filtering Options**:
  - By interface type
  - By interface name
  - Include/exclude patterns
  - Virtual interface handling

## Notifications

Notifications now include detailed information about each interface:

- Interface name and type
- Current IP addresses (IPv4/IPv6)
- Interface status
- Network statistics
- Bandwidth utilization
- Error counts

### Email Notifications

Emails include:

- Per-interface status sections
- Network statistics tables
- Bandwidth graphs
- Status indicators

### Telegram Notifications

Telegram messages include:

- Interface-specific updates
- Network statistics
- Status changes
- Bandwidth information

## Project Structure

```text
.
├── LICENSE                 # License file
├── README.md              # Project documentation
├── config.example.json    # Example configuration file
├── config.json            # Active configuration file
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── main.go                # Application entry point
├── config/                # Configuration management
│   └── config.go          # Configuration structures and validation
├── logs/                  # Log files directory
│   ├── wameter.log        # Application logs
│   └── last_ip.json      # Last known IP state
├── metrics/               # Metrics collection
│   ├── metrics.go         # Core metrics implementation
│   └── network.go         # Network-specific metrics
├── monitor/               # Core monitoring logic
│   ├── monitor.go         # Main monitor implementation
│   ├── network.go         # Network interface monitoring
│   └── network_stats.go   # Network statistics collection
├── notifier/              # Notification systems
│   ├── notifier.go        # Notification interface and manager
│   ├── email.go           # Email notification implementation
│   └── telegram.go        # Telegram notification implementation
├── scripts/               # Utility scripts
│   ├── build-vib.sh       # VIB package build script for ESXi
│   ├── install.sh         # Installation script for Linux/macOS
│   └── uninstall.sh       # Uninstallation script for Linux/macOS
├── types/                 # Data types
│   └── types.go           # Common data types
└── utils/                 # Utility functions
    ├── network.go         # Network-related utilities
    └── utils.go           # Common utilities and helpers
```

### Key Components

- **monitor/**: Core monitoring functionality

  - `monitor.go`: Main monitoring logic
  - `network.go`: Network interface detection and monitoring
  - `network_stats.go`: Network statistics collection and analysis

- **metrics/**: Metrics collection and management

  - `metrics.go`: Core metrics handling
  - `network.go`: Network-specific metrics collection

- **notifier/**: Notification systems

  - `notifier.go`: Notification management
  - `email.go`: Email notification service
  - `telegram.go`: Telegram notification service

- **utils/**: Utility functions

  - `network.go`: Network-related utilities and helpers
  - `utils.go`: Common utility functions

- **types/**: Common data structures

  - `types.go`: Shared data types and interfaces

- **config/**: Configuration handling
  - `config.go`: Configuration parsing and validation

## Prerequisites

For building:

- Go 1.20 or later
- Git

## Building

### Standard Binary

Build for your current platform:

```bash
go build -o wameter
```

Cross compilation:

For Linux:

```bash
GOOS=linux GOARCH=amd64 go build -o wameter-linux
```

For macOS:

```bash
GOOS=darwin GOARCH=amd64 go build -o wameter-macos
```

For Windows:

```bash
GOOS=windows GOARCH=amd64 go build -o wameter.exe
```

### ESXi VIB Package

```bash
./scripts/build-vib.sh
```

## Installation

### Standard Platforms (Linux/macOS/Windows)

1. Download or build the appropriate binary for your platform.
2. Create a configuration directory:

   ```bash
   # Linux/macOS
   sudo mkdir -p /etc/wameter

   # Windows (PowerShell as Administrator)
   New-Item -Path "C:\ProgramData\wameter" -ItemType Directory
   ```

3. Copy the configuration file:

   ```bash
   # Linux/macOS
   sudo cp config.example.json /etc/wameter/config.json

   # Windows
   Copy-Item config.example.json C:\ProgramData\wameter\config.json
   ```

4. Create a log directory:

   ```bash
   # Linux/macOS
   sudo mkdir -p /var/log/wameter

   # Windows
   New-Item -Path "C:\ProgramData\wameter\logs" -ItemType Directory
   ```

### ESXi Installation

1. Copy the VIB package to your ESXi host:

   ```bash
   scp dist/com.haiyon.wameter-1.0.0-1.vib root@esxi-host:/tmp/
   ```

2. Install the VIB:

   ```bash
   esxcli software vib install -v /tmp/com.haiyon.wameter-1.0.0-1.vib
   ```

## Configuration

Default configuration paths:

- Linux/macOS: `/etc/wameter/config.json`
- Windows: `C:\ProgramData\wameter\config.json`
- ESXi: `/etc/wameter/config.json`

Example configuration: `config.example.json`

### Interface Configuration Options

- `include_virtual`: Whether to monitor virtual interfaces
- `exclude_interfaces`: List of interface names or patterns to exclude
- `interface_types`: List of interface types to monitor
- `stat_collection`: Network statistics collection configuration

### Statistics Collection Options

- `enabled`: Enable/disable statistics collection
- `interval`: Collection interval in seconds
- `include_stats`: List of statistics to collect

## Running the Service

### Linux/macOS

Running directly:

```bash
./wameter -config /etc/wameter/config.json
```

Using systemd (Linux):

```bash
# Create a service file
sudo tee /etc/systemd/system/wameter.service << EOF
[Unit]
Description=Wameter Service
After=network.target

[Service]
ExecStart=/usr/local/bin/wameter -config /etc/wameter/config.json
Restart=always
User=nobody

[Install]
WantedBy=multi-user.target
EOF

# Start the service
sudo systemctl enable wameter
sudo systemctl start wameter
```

### Windows

Running directly:

```powershell
.\wameter.exe -config C:\ProgramData\wameter\config.json
```

Install as a Windows Service:

```powershell
# Using NSSM (Non-Sucking Service Manager)
nssm install Wameter "C:\Program Files\wameter\wameter.exe"
nssm set Wameter AppParameters "-config C:\ProgramData\wameter\config.json"
nssm start Wameter
```

### ESXi

Using service commands:

```bash
/etc/init.d/wameter start
/etc/init.d/wameter stop
/etc/init.d/wameter status
```

## Logging

Log file locations:

- Linux/macOS: `/var/log/wameter/monitor.log`
- Windows: `C:\ProgramData\wameter\logs\monitor.log`
- ESXi: `/var/log/wameter/monitor.log`

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
