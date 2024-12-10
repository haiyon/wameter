package utils

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wameter/internal/types"
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

// GetInterfaceStats retrieves interface statistics
func GetInterfaceStats(name string) (*types.InterfaceStats, error) {
	// Only supported on Linux
	if !IsLinux() {
		return nil, nil
	}

	stats := &types.InterfaceStats{
		CollectedAt: time.Now(),
	}

	// Get interface
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface: %w", err)
	}

	// Set basic information
	stats.IsUp = iface.Flags&net.FlagUp != 0
	stats.OperState = getOperState(name)
	stats.Speed = getInterfaceSpeed(name)
	stats.HasCarrier = hasCarrier(name)

	if err := getLinuxStats(name, stats); err != nil {
		return nil, err
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

func getOperState(name string) string {
	if !IsLinux() {
		return ""
	}

	data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func getInterfaceSpeed(name string) int64 {
	if !IsLinux() {
		return 0
	}

	data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", name))
	if err != nil {
		return 0
	}

	speed, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return speed
}

func hasCarrier(name string) bool {
	if !IsLinux() {
		return false
	}

	data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/carrier", name))
	if err != nil {
		return false
	}

	carrier, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return false
	}
	return carrier == 1
}

func getLinuxStats(name string, stats *types.InterfaceStats) error {
	statsDir := fmt.Sprintf("/sys/class/net/%s/statistics", name)

	// Read statistics files
	statFiles := map[string]*uint64{
		"rx_bytes":   &stats.RxBytes,
		"tx_bytes":   &stats.TxBytes,
		"rx_packets": &stats.RxPackets,
		"tx_packets": &stats.TxPackets,
		"rx_errors":  &stats.RxErrors,
		"tx_errors":  &stats.TxErrors,
		"rx_dropped": &stats.RxDropped,
		"tx_dropped": &stats.TxDropped,
	}

	for filename, ptr := range statFiles {
		path := filepath.Join(statsDir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		if value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			*ptr = value
		}
	}

	return nil
}
