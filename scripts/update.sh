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

update() {
    log_info "Updating wameter-$COMPONENT to version $VERSION..."

    # Backup configuration
    backup_config "$COMPONENT"

    # Stop service
    manage_service "stop" "$COMPONENT"

    # Backup current binary
    cp "$INSTALL_BASE/bin/$BINARY_NAME" "${BACKUP_DIR}/${BINARY_NAME}.backup"

    # Update binary
    cp "$BINARY_NAME" "$INSTALL_BASE/bin/"
    chmod +x "$INSTALL_BASE/bin/$BINARY_NAME"

    # Start service
    manage_service "start" "$COMPONENT"

    # Health check
    if ! check_health "$COMPONENT"; then
        log_error "Update failed - rolling back"
        cp "${BACKUP_DIR}/${BINARY_NAME}.backup" "$INSTALL_BASE/bin/$BINARY_NAME"
        manage_service "start" "$COMPONENT"
        exit 1
    fi

    log_info "Update complete"
}

update
