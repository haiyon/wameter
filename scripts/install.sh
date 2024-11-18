#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# OS detection
OS="$(uname)"
case "$OS" in
"Darwin") IS_MACOS=1 ;;
"Linux") IS_MACOS=0 ;;
*)
    print_error "Unsupported operating system: $OS"
    exit 1
    ;;
esac

# Paths (OS specific)
if [ "$IS_MACOS" -eq 1 ]; then
    BASE_DIR="/opt/ipm"
    INSTALL_DIR="$BASE_DIR/bin"
    CONFIG_DIR="$BASE_DIR/etc"
    LOG_DIR="$BASE_DIR/log"
    LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
    LAUNCH_DAEMONS_DIR="/Library/LaunchDaemons"
else
    INSTALL_DIR="/usr/local/bin"
    CONFIG_DIR="/etc/ipm"
    LOG_DIR="/var/log/ipm"
    SYSTEMD_DIR="/etc/systemd/system"
fi

BINARY_NAME="ipm"
SERVICE_NAME="com.haiyon.ipm"

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
    if [ "$IS_MACOS" -eq 1 ]; then
        if [ "$EUID" -eq 0 ]; then
            print_warn "Running as root on macOS is not recommended"
            read -p "Continue anyway? (y/N) " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
        # Ensure /opt exists and is writable
        if [ ! -d "/opt" ]; then
            sudo mkdir -p /opt
        fi
        # Ensure current user has write permission to /opt/ipm
        if [ ! -d "$BASE_DIR" ]; then
            sudo mkdir -p "$BASE_DIR"
            sudo chown -R $(whoami) "$BASE_DIR"
        fi
    else
        if [ "$EUID" -ne 0 ]; then
            print_error "Please run as root"
            exit 1
        fi
    fi
}

# Get system architecture
get_arch() {
    local arch
    arch=$(uname -m)
    case $arch in
    x86_64) echo "amd64" ;;
    arm64) echo "arm64" ;;
    aarch64) echo "arm64" ;; # For Linux ARM64
    armv7l) echo "arm" ;;
    *) echo "amd64" ;; # Default to amd64
    esac
}

# Check and build binary
check_and_build() {
    if [ ! -f "$BINARY_NAME" ]; then
        print_info "Binary not found, building..."
        if ! command -v go &>/dev/null; then
            print_error "Go is not installed"
            exit 1
        fi

        # Set environment variables
        export GOOS=$([ "$IS_MACOS" -eq 1 ] && echo "darwin" || echo "linux")
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
    local binary_arch system_arch
    if [ "$IS_MACOS" -eq 1 ]; then
        binary_arch=$(file "$BINARY_NAME" | grep -o "x86_64\|arm64")
    else
        binary_arch=$(file "$BINARY_NAME" | grep -o "x86-64\|aarch64\|ARM")
    fi
    system_arch=$(uname -m)

    if [ "$IS_MACOS" -eq 1 ]; then
        case "$binary_arch" in
        "x86_64")
            if [ "$system_arch" != "x86_64" ]; then
                print_error "Binary architecture (x86_64) doesn't match system architecture ($system_arch)"
                rm "$BINARY_NAME"
                check_and_build
            fi
            ;;
        "arm64")
            if [ "$system_arch" != "arm64" ]; then
                print_error "Binary architecture (arm64) doesn't match system architecture ($system_arch)"
                rm "$BINARY_NAME"
                check_and_build
            fi
            ;;
        esac
    else
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
        esac
    fi
}

# Create directories
create_directories() {
    print_info "Creating directories..."
    if [ "$IS_MACOS" -eq 1 ]; then
        mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$LOG_DIR" "$LAUNCH_AGENTS_DIR"
        # Set proper permissions for macOS
        chmod 755 "$BASE_DIR"
        chmod 755 "$INSTALL_DIR" "$CONFIG_DIR" "$LOG_DIR"
    else
        mkdir -p "$CONFIG_DIR" "$LOG_DIR"
        chmod 755 "$CONFIG_DIR" "$LOG_DIR"
    fi
}

# Get default interface
get_default_interface() {
    local interface=""

    if [ "$IS_MACOS" -eq 1 ]; then
        # macOS specific network interface detection
        interface=$(route -n get default 2>/dev/null | grep interface | awk '{print $2}')
        if [ -z "$interface" ]; then
            interface=$(netstat -rn | grep default | head -n1 | awk '{print $NF}')
        fi
    else
        # Linux interface detection
        if command -v ip &>/dev/null; then
            interface=$(ip route | grep default | head -n1 | awk '{print $5}')
        fi

        if [ -z "$interface" ] && command -v route &>/dev/null; then
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
    fi

    if [ -z "$interface" ]; then
        interface=$([ "$IS_MACOS" -eq 1 ] && echo "en0" || echo "eth0")
        print_warn "No active network interface found, using default: $interface"
    fi

    echo "$interface"
}

# Install files
install_files() {
    print_info "Installing binary..."
    if [ "$IS_MACOS" -eq 1 ]; then
        cp "$BINARY_NAME" "$INSTALL_DIR/"
        chmod 755 "$INSTALL_DIR/$BINARY_NAME"
        # Create symlink in /usr/local/bin for easy access
        if [ ! -L "/usr/local/bin/$BINARY_NAME" ]; then
            sudo ln -sf "$INSTALL_DIR/$BINARY_NAME" "/usr/local/bin/$BINARY_NAME"
        fi
    else
        cp "$BINARY_NAME" "$INSTALL_DIR/"
        chmod 755 "$INSTALL_DIR/$BINARY_NAME"
    fi

    print_info "Installing config file..."
    if [ ! -f "$CONFIG_DIR/config.json" ]; then
        local default_interface
        default_interface=$(get_default_interface)
        print_info "Creating default config with interface: $default_interface"
        cp "config.example.json" "$CONFIG_DIR/config.json"
        sed -i.bak "s/\"network_interface\": \"[^\"]*\"/\"network_interface\": \"$default_interface\"/" "$CONFIG_DIR/config.json"
        [ "$IS_MACOS" -eq 1 ] && rm -f "$CONFIG_DIR/config.json.bak"
    else
        print_warn "Config file already exists, skipping..."
    fi
    chmod 644 "$CONFIG_DIR/config.json"
}

# Create service
create_service() {
    if [ "$IS_MACOS" -eq 1 ]; then
        print_info "Creating launchd service..."
        cat >"$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$SERVICE_NAME</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/$BINARY_NAME</string>
        <string>-config</string>
        <string>$CONFIG_DIR/config.json</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>$LOG_DIR/error.log</string>
    <key>StandardOutPath</key>
    <string>$LOG_DIR/output.log</string>
    <key>WorkingDirectory</key>
    <string>$BASE_DIR</string>
</dict>
</plist>
EOF
        chmod 644 "$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"
    else
        print_info "Creating systemd service..."
        cat >"$SYSTEMD_DIR/$SERVICE_NAME.service" <<EOF
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
    fi
}

# Start service
start_service() {
    if [ "$IS_MACOS" -eq 1 ]; then
        print_info "Loading launchd service..."
        launchctl load "$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"
        launchctl start "$SERVICE_NAME"
    else
        print_info "Reloading systemd daemon..."
        systemctl daemon-reload
        print_info "Enabling and starting service..."
        systemctl enable "$SERVICE_NAME"
        systemctl start "$SERVICE_NAME"
    fi
}

# Check service status
check_service() {
    if [ "$IS_MACOS" -eq 1 ]; then
        local service_status
        service_status=$(launchctl list | grep "$SERVICE_NAME" 2>/dev/null)
        if [ $? -eq 0 ]; then
            local pid=$(echo "$service_status" | awk '{print $1}')
            if [ "$pid" != "-" ] && [ -n "$pid" ]; then
                print_info "Service is running (PID: $pid)"
            else
                print_error "Service is loaded but not running"
                exit 1
            fi
        else
            print_error "Service failed to start"
            print_info "Checking logs for details..."
            if [ -f "$LOG_DIR/error.log" ]; then
                tail -n 5 "$LOG_DIR/error.log"
            fi
            exit 1
        fi
    else
        if ! systemctl is-active --quiet "$SERVICE_NAME"; then
            print_error "Service failed to start"
            print_info "Checking service status..."
            systemctl status "$SERVICE_NAME" --no-pager
            exit 1
        fi
        print_info "Service is running"
        systemctl status "$SERVICE_NAME" --no-pager | grep Active
    fi
}

# Cleanup
cleanup() {
    if [ -f "$BINARY_NAME" ]; then
        print_info "Cleaning up build artifacts..."
        rm "$BINARY_NAME"
    fi
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

# Main installation function
main() {
    case "$1" in
    "version")
        show_version
        exit 0
        ;;
    "")
        check_root
        check_and_build
        create_directories
        install_files
        create_service
        start_service
        check_service
        cleanup
        print_info "Installation completed successfully!"
        ;;
    *)
        print_error "Unknown command: $1"
        echo "Usage: $0 [version]"
        exit 1
        ;;
    esac
}

# Entry point
main "$@"
