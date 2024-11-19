# Wameter

Wameter is a cross-platform IP monitoring tool that tracks internal and external IP address changes and sends notifications through multiple channels. It supports Linux, macOS, and Windows.

## Features

- **Cross-platform compatibility**: Linux, macOS, Windows
- **Network interface monitoring**:
  - Multi-interface support
  - Virtual interface monitoring (optional)
  - Real-time statistics collection
  - Status monitoring and error tracking
- **External IP tracking**: Uses multiple providers
- **Notifications**:
  - Email notifications
  - Telegram notifications
- **Customizable settings**:
  - Check intervals
  - Interface filtering
  - Notification content
- **Low resource usage**

## Quick Installation

### Steps

1. **Download or build the binary**:
   Build a binary for your platform with:

   ```bash
   go build -o wameter
   ```

2. **Set up configuration**:
   Create the configuration file directory:

   ```bash
   # Linux/macOS
   sudo mkdir -p /etc/wameter
   sudo cp config.example.json /etc/wameter/config.json

   # Windows (Run PowerShell as Administrator)
   New-Item -Path "C:\ProgramData\wameter" -ItemType Directory
   Copy-Item config.example.json C:\ProgramData\wameter\config.json
   ```

3. **Start the service**:

- On Linux/macOS:

  ```bash
  ./wameter -config /etc/wameter/config.json
  ```

- On Windows:

  ```powershell
  .\wameter.exe -config C:\ProgramData\wameter\config.json
  ```

## Configuration

Configuration file example: `config.example.json`

Key options:

- `include_virtual`: Enable/disable monitoring of virtual interfaces
- `exclude_interfaces`: List of interface names or patterns to exclude
- `interface_types`: Types of interfaces to monitor
- `stat_collection`: Options for statistics collection

## Notifications

- **Email**: Sends detailed updates with per-interface statistics
- **Telegram**: Provides real-time updates on IP changes, bandwidth, and errors

## Logging

Default log file locations:

- **Linux/macOS**: `/var/log/wameter/monitor.log`
- **Windows**: `C:\ProgramData\wameter\logs\monitor.log`

## Building

To compile the binary for your current platform:

```bash
go build -o wameter
```

To cross-compile:

- For Linux:

  ```bash
  GOOS=linux GOARCH=amd64 go build -o wameter-linux
  ```

- For macOS:

  ```bash
  GOOS=darwin GOARCH=amd64 go build -o wameter-macos
  ```

- For Windows:

  ```bash
  GOOS=windows GOARCH=amd64 go build -o wameter.exe
  ```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
