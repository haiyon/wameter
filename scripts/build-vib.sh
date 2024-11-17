#!/bin/bash

# Build script for IP Monitor VIB package

set -e

# Configuration
NAME="com.haiyon.ipm"
VENDOR="Shen"
DESCRIPTION="IP Address Monitoring Service for ESXi"
VERSION="1.0.0"
BUILD="1"
ACCEPTANCE_LEVEL="community"

# Directories
ROOT_DIR="$(pwd)"
BUILD_DIR="${ROOT_DIR}/build"
STAGE_DIR="${BUILD_DIR}/stage"
PAYLOAD_DIR="${BUILD_DIR}/payload"

# File paths and permissions
BINARY_PATH="/opt/ipm/sbin/ipm"
CONFIG_PATH="/etc/ipm/config.json"
LOG_DIR="/var/log/ipm"
PID_FILE="/var/run/ipm.pid"
INIT_SCRIPT="/etc/init.d/ipm"

# Cleanup function
cleanup() {
	echo "Cleaning up build directory..."
	rm -rf "${BUILD_DIR}"
}

# Error handler
handle_error() {
	echo "Error occurred on line $1"
	cleanup
	exit 1
}

trap 'handle_error $LINENO' ERR

# Check dependencies
check_dependencies() {
	local missing_deps=()

	# Required tools
	local tools=("go" "vibauthor" "tar" "gzip")

	for tool in "${tools[@]}"; do
		if ! command -v "$tool" >/dev/null 2>&1; then
			missing_deps+=("$tool")
		fi
	done

	if [ ${#missing_deps[@]} -ne 0 ]; then
		echo "Missing required dependencies: ${missing_deps[*]}"
		echo "Please install them before continuing."
		exit 1
	fi

	# Check Go version
	local go_version
	go_version=$(go version | awk '{print $3}' | sed 's/go//')
	# shellcheck disable=SC2072
	if [[ "${go_version}" < "1.20" ]]; then
		echo "Go version 1.20 or higher is required (current: ${go_version})"
		exit 1
	fi
}

# Create directory structure
create_directories() {
	echo "Creating directory structure..."
	mkdir -p "${STAGE_DIR}"
	mkdir -p "${PAYLOAD_DIR}/opt/ipm/sbin"
	mkdir -p "${PAYLOAD_DIR}/etc/ipm"
	mkdir -p "${PAYLOAD_DIR}/var/log/ipm"
	mkdir -p "${PAYLOAD_DIR}/etc/init.d"
}

# Build Go binary
build_binary() {
	echo "Building IP Monitor binary..."
	export GOOS=linux
	export GOARCH=amd64
	export CGO_ENABLED=0

	# Get current git commit hash and build date
	GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
	BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

	go build -o "${PAYLOAD_DIR}${BINARY_PATH}" \
		-ldflags "-s -w \
							-X main.Version=${VERSION} \
							-X main.GitCommit=${GIT_COMMIT} \
							-X main.BuildDate=${BUILD_DATE}" \
		./main.go

	# Strip binary
	strip "${PAYLOAD_DIR}${BINARY_PATH}"

	# Set permissions
	chmod 0755 "${PAYLOAD_DIR}${BINARY_PATH}"
}

# Create init script
create_init_script() {
	cat > "${PAYLOAD_DIR}${INIT_SCRIPT}" << 'EOF'
#!/bin/sh

# IP Monitor Service for ESXi
# chkconfig: 2345 90 10
# description: IP Monitor Service for ESXi hosts

### BEGIN INIT INFO
# Provides:          ipm
# Required-Start:    $network
# Required-Stop:     $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: IP Monitor Service
### END INIT INFO

DAEMON=/opt/ipm/sbin/ipm
CONFIG=/etc/ipm/config.json
PID_FILE=/var/run/ipm.pid
LOG_DIR=/var/log/ipm

# Source ESXi functions
if [ -f /etc/rc.status ]; then
	. /etc/rc.status
fi

# Ensure directories exist
ensure_dirs() {
	mkdir -p "$LOG_DIR"
	chmod 755 "$LOG_DIR"
}

# Check if process is running
check_running() {
	if [ -f "$PID_FILE" ]; then
		pid=$(cat "$PID_FILE")
		if kill -0 "$pid" 2>/dev/null; then
			return 0
		fi
	fi
	return 1
}

start() {
	if check_running; then
		echo "IP Monitor is already running."
		return 0
	fi

	ensure_dirs
	echo "Starting IP Monitor..."
	$DAEMON -config $CONFIG > /dev/null 2>&1 &
	echo $! > "$PID_FILE"

	# Verify process started
	sleep 1
	if check_running; then
		echo "IP Monitor started successfully."
		return 0
	else
		echo "Failed to start IP Monitor."
		return 1
	fi
}

stop() {
	if ! check_running; then
		echo "IP Monitor is not running."
		return 0
	fi

	echo "Stopping IP Monitor..."
	if [ -f "$PID_FILE" ]; then
		pid=$(cat "$PID_FILE")
		kill "$pid"

		# Wait for process to stop
		for i in $(seq 1 30); do
			if ! kill -0 "$pid" 2>/dev/null; then
				rm -f "$PID_FILE"
				echo "IP Monitor stopped."
				return 0
			fi
			sleep 1
		done

		# Force kill if still running
		echo "Force stopping IP Monitor..."
		kill -9 "$pid" 2>/dev/null
		rm -f "$PID_FILE"
	fi
}

status() {
	if check_running; then
		pid=$(cat "$PID_FILE")
		echo "IP Monitor is running (pid $pid)"
		return 0
	else
		echo "IP Monitor is not running"
		return 3
	fi
}

case "$1" in
	start)
		start
		;;
	stop)
		stop
		;;
	restart)
		stop
		sleep 1
		start
		;;
	status)
		status
		;;
	*)
		echo "Usage: $0 {start|stop|restart|status}"
		exit 1
		;;
esac

exit $?
EOF

	chmod 0755 "${PAYLOAD_DIR}${INIT_SCRIPT}"
}

# Copy and prepare configuration
prepare_config() {
	echo "Preparing configuration..."
	cp config.example.json "${PAYLOAD_DIR}${CONFIG_PATH}"
	chmod 0644 "${PAYLOAD_DIR}${CONFIG_PATH}"
}

# Create descriptor.xml
create_descriptor() {
	cat > "${STAGE_DIR}/descriptor.xml" << EOF
<vib version="5.0">
	<type>bootbank</type>
	<name>${NAME}</name>
	<version>${VERSION}-${BUILD}</version>
	<vendor>${VENDOR}</vendor>
	<summary>IP Monitor for ESXi</summary>
	<description>${DESCRIPTION}</description>
	<release-date>$(date +%Y-%m-%d)</release-date>
	<urls>
		<url>https://github.com/haiyon/ip-monitor</url>
	</urls>
	<relationships>
		<provides>
			<name>IPMonitor</name>
			<version>${VERSION}-${BUILD}</version>
		</provides>
		<requires>
			<constraint name="VMware-ESXi-8.0.0" relation=">="/>
		</requires>
	</relationships>
	<software-tags>
		<tag>service</tag>
	</software-tags>
	<system-requires>
		<maintenance-mode>false</maintenance-mode>
	</system-requires>
	<file-list>
		<file>${BINARY_PATH}</file>
		<file>${CONFIG_PATH}</file>
		<file>${INIT_SCRIPT}</file>
	</file-list>
	<acceptance-level>${ACCEPTANCE_LEVEL}</acceptance-level>
	<live-install-allowed>true</live-install-allowed>
	<live-remove-allowed>true</live-remove-allowed>
	<cimom-restart>false</cimom-restart>
	<stateless-ready>true</stateless-ready>
	<overlay>true</overlay>
</vib>
EOF
}

# Build VIB package
build_vib() {
	echo "Creating VIB package..."

	# Create payload archive
	cd "${PAYLOAD_DIR}"
	tar czf "${STAGE_DIR}/payload.tgz" ./*

	# Create VIB
	cd "${STAGE_DIR}"
	vibauthor -C -t . -v "${NAME}-${VERSION}-${BUILD}.vib" -f

	# Copy final VIB to output directory
	mkdir -p "${ROOT_DIR}/dist"
	cp "${NAME}-${VERSION}-${BUILD}.vib" "${ROOT_DIR}/dist/"

	echo "VIB package created: dist/${NAME}-${VERSION}-${BUILD}.vib"
}

# Main build process
main() {
	echo "Starting build process for IP Monitor VIB..."

	# Check dependencies
	check_dependencies

	# Clean previous build
	cleanup

	# Build steps
	create_directories
	build_binary
	create_init_script
	prepare_config
	create_descriptor
	build_vib

	echo "Build completed successfully!"
}

main "$@"
