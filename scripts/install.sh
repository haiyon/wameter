#!/bin/bash
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

usage() {
    echo "Usage: $0 -c <agent|server> [-v version]"
    exit 1
}

while getopts "c:v:" opt; do
    case $opt in
        c) COMPONENT="$OPTARG" ;;
        v) VERSION="$OPTARG" ;;
        *) usage ;;
    esac
done

[[ -z "$COMPONENT" || ! "$COMPONENT" =~ ^(agent|server)$ ]] && usage
VERSION="${VERSION:-latest}"

get_component_paths "$COMPONENT"
check_root

install() {
    log_info "Installing wameter-$COMPONENT $VERSION..."

    # Create directories
    run_privileged mkdir -p "$INSTALL_BASE/bin" "$(dirname "$CONFIG_FILE")" "$(dirname "$LOG_FILE")" "$BACKUP_DIR"
    [[ "$COMPONENT" = "agent" ]] && run_privileged mkdir -p "$CACHE_DIR" || run_privileged mkdir -p "$DB_DIR"

    # Copy binary
    run_privileged cp "$BINARY_NAME" "$INSTALL_BASE/bin/"
    run_privileged chmod +x "$INSTALL_BASE/bin/$BINARY_NAME"

    # Copy config
    run_privileged cp "${SCRIPT_DIR}/../examples/${COMPONENT}.example.yaml" "$CONFIG_FILE"

    # Install service
    if [[ "$(detect_os)" == "linux" ]]; then
        run_privileged cp "${SCRIPT_DIR}/../examples/wameter-${COMPONENT}.service" "/etc/systemd/system/"
        run_privileged systemctl daemon-reload
        run_privileged systemctl enable "wameter-$COMPONENT"
    else
        run_privileged cp "${SCRIPT_DIR}/../examples/com.wameter.${COMPONENT}.plist" "/Library/LaunchDaemons/"
    fi

    manage_service "start" "$COMPONENT"
    log_info "Installation complete"
}

install
