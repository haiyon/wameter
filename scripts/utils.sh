#!/bin/bash
set -e

# Colors and formatting
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Check if running with root/sudo
check_root() {
    local os=$(detect_os)
    if [ $EUID -ne 0 ]; then
        case "$os" in
            darwin)
                log_warn "Root privileges required. Please enter your password when prompted."
                if ! sudo -v; then
                    log_error "Failed to obtain root privileges"
                    exit 1
                fi
                # Keep sudo timestamp updated
                while true; do sudo -n true; sleep 60; kill -0 "$$" || exit; done 2>/dev/null &
                ;;
            linux)
                if command -v sudo >/dev/null 2>&1; then
                    log_warn "Root privileges required. Please enter your password when prompted."
                    if ! sudo -v; then
                        log_error "Failed to obtain root privileges"
                        exit 1
                    fi
                    # Keep sudo timestamp updated
                    while true; do sudo -n true; sleep 60; kill -0 "$$" || exit; done 2>/dev/null &
                else
                    log_error "This script must be run as root or with sudo"
                    exit 1
                fi
                ;;
        esac
    fi
}

# Execute command with appropriate privileges
run_privileged() {
    if [ $EUID -eq 0 ]; then
        "$@"
    else
        sudo "$@"
    fi
}

get_component_paths() {
    local component="$1"
    local os=$(detect_os)

    INSTALL_BASE="/opt/wameter"
    BINARY_NAME="$component"
    BACKUP_DIR="${INSTALL_BASE}/backups"

    case "$os" in
        linux)
            CONFIG_FILE="/etc/wameter/${component}.yaml"
            LOG_FILE="/var/log/wameter/${component}.log"
            case "$component" in
                agent)
                    CACHE_DIR="/var/lib/wameter/cache"
                    ;;
                server)
                    DB_DIR="/var/lib/wameter/db"
                    ;;
            esac
            ;;
        darwin)
            CONFIG_FILE="${INSTALL_BASE}/etc/${component}.yaml"
            LOG_FILE="${INSTALL_BASE}/log/${component}.log"
            case "$component" in
                agent)
                    CACHE_DIR="${INSTALL_BASE}/data/cache"
                    ;;
                server)
                    DB_DIR="${INSTALL_BASE}/data/db"
                    ;;
            esac
            ;;
    esac

    export INSTALL_BASE BINARY_NAME CONFIG_FILE LOG_FILE BACKUP_DIR
    [[ "$component" = "agent" ]] && export CACHE_DIR || export DB_DIR
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *) log_error "Unsupported OS"; exit 1 ;;
    esac
}

manage_service() {
    local action="$1"
    local service="$2"
    local os=$(detect_os)

    case "$os" in
        linux)
            run_privileged systemctl "$action" "$service"
            ;;
        darwin)
            if [[ "$action" == "start" ]]; then
                run_privileged launchctl load "/Library/LaunchDaemons/com.wameter.${service}.plist"
            elif [[ "$action" == "stop" ]]; then
                run_privileged launchctl unload "/Library/LaunchDaemons/com.wameter.${service}.plist"
            fi
            ;;
    esac
}

is_service_active() {
    local service="$1"
    local os=$(detect_os)

    case "$os" in
        linux)
            run_privileged systemctl is-active --quiet "$service"
            ;;
        darwin)
            launchctl list | grep -q "com.wameter.$service"
            ;;
    esac
}
