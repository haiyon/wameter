#!/bin/bash
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

usage() {
    echo "Usage: $0 -c <agent|server>"
    exit 1
}

while getopts "c:" opt; do
    case $opt in
        c) COMPONENT="$OPTARG" ;;
        *) usage ;;
    esac
done

[[ -z "$COMPONENT" || ! "$COMPONENT" =~ ^(agent|server)$ ]] && usage

get_component_paths "$COMPONENT"
check_root

uninstall() {
    log_info "Uninstalling wameter-$COMPONENT..."

    # Stop service
    if is_service_active "$COMPONENT"; then
        manage_service "stop" "$COMPONENT"
    fi

    # Remove service files
    if [[ "$(detect_os)" == "linux" ]]; then
        run_privileged systemctl disable "wameter-$COMPONENT"
        run_privileged rm -f "/etc/systemd/system/wameter-${COMPONENT}.service"
        run_privileged systemctl daemon-reload
    else
        run_privileged rm -f "/Library/LaunchDaemons/com.wameter.${COMPONENT}.plist"
    fi

    # Remove files
    run_privileged rm -f "$INSTALL_BASE/bin/$BINARY_NAME"
    run_privileged rm -f "$CONFIG_FILE"
    run_privileged rm -f "$LOG_FILE"
    [[ "$COMPONENT" = "agent" ]] && run_privileged rm -rf "$CACHE_DIR" || run_privileged rm -rf "$DB_DIR"

    log_info "Uninstallation complete"
}

uninstall
