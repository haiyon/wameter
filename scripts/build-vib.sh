#!/bin/bash

set -e

# Configuration
NAME="com.wameter.monitor"
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
TEMP_DIR="/tmp/vib-temp-$$"

# File paths
BINARY_PATH="/opt/wameter/sbin/wameter"
CONFIG_PATH="/etc/wameter/config.json"
INIT_SCRIPT="/etc/init.d/wameter"

# Logger function
log() {
	echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Cleanup function
cleanup() {
	rm -rf "${BUILD_DIR}" "${TEMP_DIR}"
}

# Error handler
handle_error() {
	log "Error occurred on line $1"
	cleanup
	exit 1
}

trap 'handle_error $LINENO' ERR
trap 'cleanup' EXIT

# Check required tools
check_dependencies() {
	local tools=("go" "tar" "gzip" "ar" "sha256sum" "sha1sum")
	local missing=()

	for tool in "${tools[@]}"; do
		command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
	done

	if [ ${#missing[@]} -ne 0 ]; then
		log "Missing required tools: ${missing[*]}"
		exit 1
	fi

	# Verify Go version
	local go_version
	go_version=$(go version | awk '{print $3}' | sed 's/go//')
	required_version="1.20"
	if [[ $(echo "$go_version $required_version" | awk '{print ($1 < $2)}') -eq 1 ]]; then
		log "Go version 1.20 or higher required (current: ${go_version})"
		exit 1
	fi
}

# Create directory structure
create_directories() {
	mkdir -p "${STAGE_DIR}"
	mkdir -p "${PAYLOAD_DIR}/opt/wameter/sbin"
	mkdir -p "${PAYLOAD_DIR}/etc/wameter"
	mkdir -p "${PAYLOAD_DIR}/var/log/wameter"
	mkdir -p "${PAYLOAD_DIR}/etc/init.d"
	mkdir -p "${TEMP_DIR}"
}

# Build Go binary
build_binary() {
	export GOOS=linux GOARCH=amd64 CGO_ENABLED=0

	GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
	BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

	go build -o "${PAYLOAD_DIR}${BINARY_PATH}" \
		-ldflags "-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" \
		./main.go

	chmod 0755 "${PAYLOAD_DIR}${BINARY_PATH}"
}

# Create init script
create_init_script() {
	cat >"${PAYLOAD_DIR}${INIT_SCRIPT}" <<'EOF'
#!/bin/sh

# Wameter Service for ESXi
# chkconfig: 2345 90 10

DAEMON=/opt/wameter/sbin/wameter
CONFIG=/etc/wameter/config.json
PID_FILE=/var/run/wameter.pid
LOG_DIR=/var/log/wameter

[ -f /etc/rc.status ] && . /etc/rc.status

ensure_dirs() {
	mkdir -p "$LOG_DIR" && chmod 755 "$LOG_DIR"
}

check_running() {
	[ -f "$PID_FILE" ] && kill -0 $(cat "$PID_FILE") 2>/dev/null
}

start() {
	if check_running; then
		echo "Wameter is already running."
		return 0
	fi

	ensure_dirs
	$DAEMON -config $CONFIG > /dev/null 2>&1 &
	echo $! > "$PID_FILE"

	sleep 1
	if check_running; then
		echo "Wameter started successfully."
		return 0
	fi
	echo "Failed to start Wameter."
	return 1
}

stop() {
	if ! check_running; then
		echo "Wameter is not running."
		return 0
	fi

	pid=$(cat "$PID_FILE")
	kill "$pid"
	for i in $(seq 1 30); do
		if ! kill -0 "$pid" 2>/dev/null; then
			rm -f "$PID_FILE"
			echo "Wameter stopped."
			return 0
		fi
		sleep 1
	done

	kill -9 "$pid" 2>/dev/null
	rm -f "$PID_FILE"
	echo "Force stopped Wameter."
}

status() {
	if check_running; then
		echo "Wameter is running (pid $(cat "$PID_FILE"))"
		return 0
	fi
	echo "Wameter is not running"
	return 3
}

case "$1" in
	start) start ;;
	stop) stop ;;
	restart) stop; sleep 1; start ;;
	status) status ;;
	*) echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac

exit $?
EOF

	chmod 0755 "${PAYLOAD_DIR}${INIT_SCRIPT}"
}

# Prepare configuration
prepare_config() {
	cp config.example.json "${PAYLOAD_DIR}${CONFIG_PATH}"
	chmod 0644 "${PAYLOAD_DIR}${CONFIG_PATH}"
}

# Build VIB package
build_vib() {
	# Create and verify payload
	cd "${PAYLOAD_DIR}"
	tar czf "${TEMP_DIR}/payload1" ./*

	# Calculate payload information
	PAYLOAD_FILES=$(tar tf "${TEMP_DIR}/payload1" | grep -v -E '/$' | sed -e 's/^/    <file>/' -e 's/$/<\/file>/')
	PAYLOAD_SIZE=$(stat -c %s "${TEMP_DIR}/payload1")
	PAYLOAD_SHA256=$(sha256sum "${TEMP_DIR}/payload1" | awk '{print $1}')
	PAYLOAD_SHA256_ZCAT=$(zcat "${TEMP_DIR}/payload1" | sha256sum | awk '{print $1}')
	PAYLOAD_SHA1_ZCAT=$(zcat "${TEMP_DIR}/payload1" | sha1sum | awk '{print $1}')

	# Create descriptor.xml
	cat > "${TEMP_DIR}/descriptor.xml" << EOF
<vib version="5.0">
    <type>bootbank</type>
    <name>${NAME}</name>
    <version>${VERSION}-${BUILD}</version>
    <vendor>${VENDOR}</vendor>
    <summary>Wameter for ESXi</summary>
    <description>${DESCRIPTION}</description>
    <release-date>$(date -u '+%Y-%m-%dT%H:%M:%S')</release-date>
    <urls>
        <url key="website">https://github.com/haiyon/wameter</url>
    </urls>
    <relationships>
        <provides>
            <name>IPMonitor</name>
            <version>${VERSION}-${BUILD}</version>
        </provides>
        <depends/>
        <conflicts/>
        <replaces/>
        <compatibleWith/>
    </relationships>
    <software-tags>
        <tag>service</tag>
    </software-tags>
    <system-requires>
        <maintenance-mode>false</maintenance-mode>
    </system-requires>
    <file-list>
${PAYLOAD_FILES}
    </file-list>
    <acceptance-level>${ACCEPTANCE_LEVEL}</acceptance-level>
    <live-install-allowed>false</live-install-allowed>
    <live-remove-allowed>true</live-remove-allowed>
    <cimom-restart>false</cimom-restart>
    <stateless-ready>true</stateless-ready>
    <overlay>false</overlay>
    <vibType>bootbank</vibType>
    <payloads>
        <payload name="payload1" type="tgz" size="${PAYLOAD_SIZE}" installSize="${PAYLOAD_SIZE}">
            <checksum checksum-type="sha-256">${PAYLOAD_SHA256}</checksum>
            <checksum checksum-type="sha-256" verify-process="gunzip">${PAYLOAD_SHA256_ZCAT}</checksum>
            <checksum checksum-type="sha-1" verify-process="gunzip">${PAYLOAD_SHA1_ZCAT}</checksum>
        </payload>
    </payloads>
</vib>
EOF

	# Create VIB package
	touch "${TEMP_DIR}/sig.pkcs7"
	mkdir -p "${ROOT_DIR}/dist"

	cd "${TEMP_DIR}"
	VIB_FILE="${NAME}-${VERSION}-${BUILD}.vib"
	ar r "${VIB_FILE}" descriptor.xml sig.pkcs7 payload1
	mv "${VIB_FILE}" "${ROOT_DIR}/dist/"

	log "VIB package created: dist/${VIB_FILE}"
}

# Verify VIB package
verify_vib() {
	local vib_file="dist/${NAME}-${VERSION}-${BUILD}.vib"
	[ -f "$vib_file" ] && [ -s "$vib_file" ] && log "VIB verification passed"
}

# Main process
main() {
	check_dependencies
	cleanup
	create_directories
	build_binary
	create_init_script
	prepare_config
	build_vib
	verify_vib || exit 1
	log "Build completed successfully"
}

main "$@"
