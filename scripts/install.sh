#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Paths
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/ipm"
LOG_DIR="/var/log/ipm"
SYSTEMD_DIR="/etc/systemd/system"
BINARY_NAME="ipm"
SERVICE_NAME="ipm"

# Print functions
print_info() {
	echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
	echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
	echo -e "${RED}[ERROR]${NC} $1"
}

# Check root
check_root() {
	if [ "$EUID" -ne 0 ]; then
		print_error "Please run as root"
		exit 1
	fi
}

# Get system architecture
get_arch() {
	local arch=$(uname -m)
	case $arch in
		x86_64) echo "amd64" ;;
		aarch64) echo "arm64" ;;
		armv7l) echo "arm" ;;
		*) echo "amd64" ;; # Default to amd64
	esac
}

# Check and build binary
check_and_build() {
	if [ ! -f "$BINARY_NAME" ]; then
		print_info "Binary not found, building..."
		if ! command -v go &> /dev/null; then
			print_error "Go is not installed"
			exit 1
		fi

		# Set environment variables
		export GOOS=linux
		export GOARCH=$(get_arch)
		export CGO_ENABLED=0

		# Get current git commit hash and build date
		VERSION=${VERSION:-"1.0.0"}
		GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
		BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

		print_info "Building for GOOS=$GOOS GOARCH=$GOARCH"
		print_info "Version: $VERSION"
		print_info "Git commit: $GIT_COMMIT"
		print_info "Build date: $BUILD_DATE"

		# Build binary
		if ! go build -o "$BINARY_NAME" \
			-ldflags "-s -w \
							  -X main.Version=${VERSION} \
							  -X main.GitCommit=${GIT_COMMIT} \
							  -X main.BuildDate=${BUILD_DATE}"; then
			print_error "Build failed"
			exit 1
		fi

		print_info "Build successful"
	else
		print_info "Binary already exists, skipping build"
		check_binary_arch
	fi
}

# Check binary architecture
check_binary_arch() {
	local binary_arch=$(file "$BINARY_NAME" | grep -o "x86-64\|aarch64\|ARM")
	local system_arch=$(uname -m)

	case "$binary_arch" in
		"x86-64")
			if [ "$system_arch" != "x86_64" ]; then
				print_error "Binary architecture (x86-64) doesn't match system architecture ($system_arch)"
				rm "$BINARY_NAME"
				check_and_build
			fi
			;;
		"aarch64")
			if [ "$system_arch" != "aarch64" ]; then
				print_error "Binary architecture (aarch64) doesn't match system architecture ($system_arch)"
				rm "$BINARY_NAME"
				check_and_build
			fi
			;;
		"ARM")
			if [[ "$system_arch" != "armv"* ]]; then
				print_error "Binary architecture (ARM) doesn't match system architecture ($system_arch)"
				rm "$BINARY_NAME"
				check_and_build
			fi
			;;
		*)
			print_warn "Unable to determine binary architecture, proceeding anyway..."
			;;
	esac
}

# Create directories
create_directories() {
	print_info "Creating directories..."
	mkdir -p "$CONFIG_DIR" "$LOG_DIR"
	chmod 755 "$CONFIG_DIR" "$LOG_DIR"
}

# Get default interface
get_default_interface() {
	local interface=""

	if command -v ip &> /dev/null; then
		interface=$(ip route | grep default | head -n1 | awk '{print $5}')
	fi

	 if [ -z "$interface" ] && command -v route &> /dev/null; then
		interface=$(route -n | grep '^0.0.0.0' | head -n1 | awk '{print $8}')
	fi

	if [ -z "$interface" ]; then
		for iface in eth0 eth1 en0 en1 ens32 ens33 enp0s3 enp0s8; do
			if [ -d "/sys/class/net/$iface" ] && [ "$(cat /sys/class/net/$iface/operstate)" = "up" ]; then
				interface=$iface
				break
			fi
		done
	fi

	if [ -z "$interface" ]; then
		interface=$(ls /sys/class/net/ | grep -v lo | head -n1)
	fi

	if [ -z "$interface" ]; then
		interface="eth0"
		print_warn "No active network interface found, using default: eth0" >&2
	fi

	echo "$interface"
}

# Install files
install_files() {
	print_info "Installing binary..."
	cp "$BINARY_NAME" "$INSTALL_DIR/"
	chmod 755 "$INSTALL_DIR/$BINARY_NAME"

	print_info "Installing config file..."
	if [ ! -f "$CONFIG_DIR/config.json" ]; then
		# If config file doesn't exist, create it, full config reference: config.example.json
		local default_interface=$(get_default_interface)
		print_info "Creating default config with interface: $default_interface"
		cat > "$CONFIG_DIR/config.json" <<EOF
{
  "check_interval": 300,
  "network_interface": "${default_interface}",
  "ip_version": {
    "enable_ipv4": true,
    "enable_ipv6": true,
    "prefer_ipv6": false
  },
  "check_external_ip": true,
  "external_ip_providers": {
    "ipv4": [
      "https://api.ipify.org",
      "https://ifconfig.me/ip",
      "https://icanhazip.com"
    ],
    "ipv6": [
      "https://api6.ipify.org",
      "https://v6.ident.me",
      "https://api6.my-ip.io/ip"
    ]
  },
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
    "directory": "/var/log/ipm",
    "filename": "ip_monitor.log",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true,
    "level": "info"
  },
  "retry": {
    "max_attempts": 3,
    "initial_delay": "1s",
    "max_delay": "30s"
  },
  "monitoring": {
    "http_timeout": "10s",
    "dial_timeout": "5s",
    "keep_alive": "30s",
    "idle_timeout": "90s",
    "max_idle_conns": 100,
    "max_idle_conns_per_host": 10
  },
  "last_ip_file": "/var/log/ipm/last_ip.json",
  "debug": false,
  "security": {
    "file_mode": "0644",
    "dir_mode": "0755"
  }
}
EOF
	else
		print_warn "Config file already exists, skipping..."
	fi
	chmod 644 "$CONFIG_DIR/config.json"
}

# Create systemd service
create_service() {
	print_info "Creating systemd service..."
	cat > "$SYSTEMD_DIR/$SERVICE_NAME.service" <<EOF
[Unit]
Description=IP Monitor Service
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/$BINARY_NAME -config $CONFIG_DIR/config.json
Restart=always
RestartSec=10
User=root

[Install]
WantedBy=multi-user.target
EOF

	chmod 644 "$SYSTEMD_DIR/$SERVICE_NAME.service"
}


# Show version
show_version() {
	if [ -f "$BINARY_NAME" ]; then
		print_info "Checking binary version..."
		./"$BINARY_NAME" -version
	else
		print_error "Binary not found"
		exit 1
	fi
}

# Start service
start_service() {
	print_info "Reloading systemd daemon..."
	systemctl daemon-reload

	print_info "Enabling and starting service..."
	systemctl enable $SERVICE_NAME
	systemctl start $SERVICE_NAME
}

# Cleanup
cleanup() {
	if [ -f "$BINARY_NAME" ]; then
		print_info "Cleaning up build artifacts..."
		rm "$BINARY_NAME"
	fi
}

# Uninstall
uninstall() {
	print_info "Stopping service..."
	systemctl stop $SERVICE_NAME 2>/dev/null || true
	systemctl disable $SERVICE_NAME 2>/dev/null || true

	print_info "Removing files..."
	rm -f "$SYSTEMD_DIR/$SERVICE_NAME.service"
	rm -f "$INSTALL_DIR/$BINARY_NAME"

	print_warn "The following directories and files will not be removed:"
	print_warn "- Config directory: $CONFIG_DIR"
	print_warn "- Log directory: $LOG_DIR"
	print_warn "You can manually remove them if needed."

	systemctl daemon-reload
	print_info "Uninstallation complete"
}

# Main
main() {
	case "$1" in
		"uninstall") check_root; uninstall; exit 0 ;;
		"version") show_version; exit 0 ;;
		"") check_root; check_and_build; create_directories; install_files; create_service; start_service; check_service; cleanup
			print_info "Installation completed successfully!"
			;;
		*) print_error "Unknown command: $1"; echo "Usage: $0 [uninstall|version]"; exit 1 ;;
	esac
}

# Entry point
main "$@"
