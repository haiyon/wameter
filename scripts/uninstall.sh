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
    BASE_DIR="/opt/wameter"
    INSTALL_DIR="$BASE_DIR/bin"
    CONFIG_DIR="$BASE_DIR/etc"
    LOG_DIR="$BASE_DIR/log"
    LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
    LAUNCH_DAEMONS_DIR="/Library/LaunchDaemons"
else
    INSTALL_DIR="/usr/local/bin"
    CONFIG_DIR="/etc/wameter"
    LOG_DIR="/var/log/wameter"
    SYSTEMD_DIR="/etc/systemd/system"
fi

BINARY_NAME="wameter"
SERVICE_NAME="com.wameter.monitor"

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
    else
        if [ "$EUID" -ne 0 ]; then
            print_error "Please run as root"
            exit 1
        fi
    fi
}

# Confirm uninstall
confirm_uninstall() {
    print_warn "This will uninstall IPM and remove all its files."
    read -p "Are you sure you want to continue? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Uninstallation cancelled"
        exit 0
    fi
}

# Stop and remove service
remove_service() {
    if [ "$IS_MACOS" -eq 1 ]; then
        print_info "Stopping and unloading service..."
        launchctl stop "$SERVICE_NAME" 2>/dev/null || true
        launchctl unload "$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist" 2>/dev/null || true
        rm -f "$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"
        rm -f "/usr/local/bin/$BINARY_NAME" # Remove symlink
    else
        print_info "Stopping service..."
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
        systemctl disable "$SERVICE_NAME" 2>/dev/null || true
        rm -f "$SYSTEMD_DIR/$SERVICE_NAME.service"
        systemctl daemon-reload
    fi
}

# Remove files
remove_files() {
    if [ "$IS_MACOS" -eq 1 ]; then
        print_info "Removing application directory..."
        if [ -d "$BASE_DIR" ]; then
            sudo rm -rf "$BASE_DIR"
        fi
    else
        print_info "Removing binary..."
        rm -f "$INSTALL_DIR/$BINARY_NAME"

        print_warn "The following directories and files will not be removed:"
        print_warn "- Config directory: $CONFIG_DIR"
        print_warn "- Log directory: $LOG_DIR"
        print_warn "You can manually remove them if needed."
    fi
}

# Check if installed
check_installation() {
    local installed=0
    if [ "$IS_MACOS" -eq 1 ]; then
        if [ -d "$BASE_DIR" ] || [ -f "/usr/local/bin/$BINARY_NAME" ] || [ -f "$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist" ]; then
            installed=1
        fi
    else
        if [ -f "$INSTALL_DIR/$BINARY_NAME" ] || [ -f "$SYSTEMD_DIR/$SERVICE_NAME.service" ]; then
            installed=1
        fi
    fi

    if [ $installed -eq 0 ]; then
        print_error "IPM is not installed"
        exit 1
    fi
}

# Main uninstallation function
main() {
    check_root
    check_installation
    confirm_uninstall
    remove_service
    remove_files
    print_info "Uninstallation completed successfully!"
}

# Entry point
main "$@"
