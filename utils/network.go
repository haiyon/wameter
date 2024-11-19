package utils

import (
	"fmt"
	"ip-monitor/types"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IsValidIP checks if a string is a valid IP address, optionally checking for IPv6
func IsValidIP(ip string, wantV6 ...bool) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	if len(wantV6) > 0 && wantV6[0] {
		return parsedIP.To16() != nil && parsedIP.To4() == nil
	}
	return parsedIP.To4() != nil
}

// IsGlobalIPv6 checks if the IPv6 address is global (not link-local)
func IsGlobalIPv6(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return true
}

// IsVirtualInterface checks if the interface is virtual/non-physical
func IsVirtualInterface(name string) bool {
	// Common prefixes for virtual interfaces
	virtualPrefixes := []string{
		"docker", "veth", "br-", "vmbr", "virbr",
		"vnet", "tun", "tap", "bond", "team",
		"vmnet", "wg", "ham", "vxlan", "overlay",
	}

	name = strings.ToLower(name)
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// NetworkMaskSize returns the size of the network mask
func NetworkMaskSize(mask net.IPMask) int {
	size, _ := mask.Size()
	return size
}

// ReadNetworkStat reads a specific network interface statistic
func ReadNetworkStat(ifaceName, statName string) (uint64, error) {
	if !IsLinux() {
		return 0, fmt.Errorf("network statistics are only supported on Linux")
	}

	path := filepath.Join("/sys/class/net", ifaceName, "statistics", statName)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read network stat %s for interface %s: %w",
			statName, ifaceName, err)
	}

	// Parse the value
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse network stat %s for interface %s: %w",
			statName, ifaceName, err)
	}

	return value, nil
}

// GetInterfaceType determines the type of network interface
func GetInterfaceType(ifaceName string) InterfaceType {
	name := strings.ToLower(ifaceName)

	// Check common prefixes for different types
	for prefix, ifaceType := range interfaceTypePrefixes {
		if strings.HasPrefix(name, prefix) {
			return ifaceType
		}
	}

	// Check if it's a wireless interface (on Linux)
	if IsLinux() {
		if _, err := os.Stat(filepath.Join("/sys/class/net", ifaceName, "wireless")); err == nil {
			return InterfaceTypeWireless
		}
	}

	// Default to Ethernet if no specific type is identified
	return InterfaceTypeEthernet
}

// InterfaceType represents the type of network interface
type InterfaceType string

const (
	InterfaceTypeEthernet  InterfaceType = "ethernet"
	InterfaceTypeWireless  InterfaceType = "wireless"
	InterfaceTypeVirtual   InterfaceType = "virtual"
	InterfaceTypeBridge    InterfaceType = "bridge"
	InterfaceTypeTunnel    InterfaceType = "tunnel"
	InterfaceTypeBonding   InterfaceType = "bonding"
	InterfaceTypeContainer InterfaceType = "container"
	InterfaceTypeVPN       InterfaceType = "vpn"
)

// interfaceTypePrefixes maps interface name prefixes to their types
var interfaceTypePrefixes = map[string]InterfaceType{
	"eth":    InterfaceTypeEthernet,
	"en":     InterfaceTypeEthernet, // macOS/BSD style
	"wlan":   InterfaceTypeWireless,
	"wifi":   InterfaceTypeWireless,
	"wl":     InterfaceTypeWireless,
	"docker": InterfaceTypeContainer,
	"veth":   InterfaceTypeVirtual,
	"br":     InterfaceTypeBridge,
	"bond":   InterfaceTypeBonding,
	"tun":    InterfaceTypeTunnel,
	"tap":    InterfaceTypeTunnel,
	"vpn":    InterfaceTypeVPN,
	"wg":     InterfaceTypeVPN, // WireGuard
	"ipsec":  InterfaceTypeVPN,
	"vxlan":  InterfaceTypeVirtual,
	"vmnet":  InterfaceTypeVirtual,
	"virbr":  InterfaceTypeBridge, // libvirt bridge
	"lxcbr":  InterfaceTypeBridge, // LXC bridge
	"vmbr":   InterfaceTypeBridge, // Proxmox bridge
}

// IsPhysicalInterface checks if the interface is physical
func IsPhysicalInterface(name string, flags net.Flags) bool {
	ifaceType := GetInterfaceType(name)

	// Consider Ethernet and Wireless interfaces as physical
	isPhysical := ifaceType == InterfaceTypeEthernet || ifaceType == InterfaceTypeWireless

	// Check flags for basic requirements
	hasValidFlags := flags&net.FlagLoopback == 0 && // not loopback
		flags&net.FlagUp != 0 // is up

	return isPhysical && hasValidFlags
}

// Common network statistics file names in Linux
var networkStatFiles = []string{
	"rx_bytes",
	"tx_bytes",
	"rx_packets",
	"tx_packets",
	"rx_errors",
	"tx_errors",
	"rx_dropped",
	"tx_dropped",
	"rx_fifo_errors",
	"tx_fifo_errors",
	"rx_frame_errors",
	"tx_carrier_errors",
	"rx_compressed",
	"tx_compressed",
	"multicast",
	"collisions",
}

// GetInterfaceStats retrieves detailed statistics for a network interface
func GetInterfaceStats(ifaceName string) (*types.InterfaceStats, error) {
	stats := &types.InterfaceStats{
		CollectedAt: time.Now(),
	}

	if !IsLinux() {
		// For non-Linux systems, try to get basic interface info
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get interface %s: %w", ifaceName, err)
		}

		// Set basic information
		stats.IsUp = iface.Flags&net.FlagUp != 0
		stats.MTU = iface.MTU

		// On non-Linux systems, we return limited information
		return stats, nil
	}

	// On Linux, read detailed statistics from /sys/class/net
	basePath := filepath.Join("/sys/class/net", ifaceName, "statistics")

	// Get interface status
	if operstatePath := filepath.Join("/sys/class/net", ifaceName, "operstate"); IsFileExists(operstatePath) {
		if data, err := os.ReadFile(operstatePath); err == nil {
			stats.OperState = strings.TrimSpace(string(data))
		}
	}

	// Get speed if available
	if speedPath := filepath.Join("/sys/class/net", ifaceName, "speed"); IsFileExists(speedPath) {
		if data, err := os.ReadFile(speedPath); err == nil {
			if speed, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
				stats.Speed = speed // Speed in Mbps
			}
		}
	}

	// Get carrier status
	if carrierPath := filepath.Join("/sys/class/net", ifaceName, "carrier"); IsFileExists(carrierPath) {
		if data, err := os.ReadFile(carrierPath); err == nil {
			if carrier, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
				stats.HasCarrier = carrier == 1
			}
		}
	}

	// Read MTU
	if mtuPath := filepath.Join("/sys/class/net", ifaceName, "mtu"); IsFileExists(mtuPath) {
		if data, err := os.ReadFile(mtuPath); err == nil {
			if mtu, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
				stats.MTU = int(mtu)
			}
		}
	}

	// Read all available statistics
	for _, statFile := range networkStatFiles {
		path := filepath.Join(basePath, statFile)
		if !IsFileExists(path) {
			continue
		}

		value, err := ReadNetworkStat(ifaceName, statFile)
		if err != nil {
			continue // Skip this stat if there's an error
		}

		// Map the statistics to struct fields
		switch statFile {
		case "rx_bytes":
			stats.RxBytes = value
		case "tx_bytes":
			stats.TxBytes = value
		case "rx_packets":
			stats.RxPackets = value
		case "tx_packets":
			stats.TxPackets = value
		case "rx_errors":
			stats.RxErrors = value
		case "tx_errors":
			stats.TxErrors = value
		case "rx_dropped":
			stats.RxDropped = value
		case "tx_dropped":
			stats.TxDropped = value
		case "rx_fifo_errors":
			stats.RxFifoErrors = value
		case "tx_fifo_errors":
			stats.TxFifoErrors = value
		case "rx_frame_errors":
			stats.RxFrameErrors = value
		case "tx_carrier_errors":
			stats.TxCarrierErrors = value
		case "rx_compressed":
			stats.RxCompressed = value
		case "tx_compressed":
			stats.TxCompressed = value
		case "multicast":
			stats.Multicast = value
		case "collisions":
			stats.Collisions = value
		}
	}

	return stats, nil
}

// IsFileExists checks if a file exists
func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FormatBytes formats bytes into human readable format
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatBytesRate formats bytes rate into human readable format
func FormatBytesRate(bytesPerSec float64) string {
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.1f B", bytesPerSec)
	}
	div, exp := unit, 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		bytesPerSec/div, "KMGTPE"[exp])
}
