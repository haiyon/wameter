package metrics

import (
	"time"

	"github.com/haiyon/wameter/types"
	"github.com/haiyon/wameter/utils"
)

// NetworkInterfaceMetrics represents metrics for a single network interface
type NetworkInterfaceMetrics struct {
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	IPv4AddressCount    int       `json:"ipv4_address_count"`
	IPv6AddressCount    int       `json:"ipv6_address_count"`
	Status              string    `json:"status"`
	Speed               int64     `json:"speed_mbps"`
	LastCheck           time.Time `json:"last_check"`
	ConsecutiveFailures int64     `json:"consecutive_failures"`
	UptimeSeconds       int64     `json:"uptime_seconds"`

	// Traffic statistics
	RxBytes       uint64  `json:"rx_bytes"`
	TxBytes       uint64  `json:"tx_bytes"`
	RxBytesRate   float64 `json:"rx_bytes_rate"`
	TxBytesRate   float64 `json:"tx_bytes_rate"`
	RxPackets     uint64  `json:"rx_packets"`
	TxPackets     uint64  `json:"tx_packets"`
	RxPacketsRate float64 `json:"rx_packets_rate"`
	TxPacketsRate float64 `json:"tx_packets_rate"`

	// Error statistics
	Errors        int64     `json:"errors"`
	Dropped       int64     `json:"dropped"`
	LastError     string    `json:"last_error"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`

	// IP change history
	LastIPChange   time.Time `json:"last_ip_change"`
	TotalIPChanges int64     `json:"total_ip_changes"`
	IPv4Changes    int64     `json:"ipv4_changes"`
	IPv6Changes    int64     `json:"ipv6_changes"`
}

// UpdateNetworkStats updates network interface metrics
func (m *Metrics) UpdateNetworkStats(state *types.IPState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update metrics for each interface
	for ifaceName, ifaceState := range state.InterfaceInfo {
		// Create metrics entry if it doesn't exist
		if _, exists := m.NetworkStats[ifaceName]; !exists {
			m.NetworkStats[ifaceName] = &NetworkInterfaceMetrics{
				Name: ifaceName,
				Type: string(utils.GetInterfaceType(ifaceName)),
			}
		}

		stats := m.NetworkStats[ifaceName]
		stats.IPv4AddressCount = len(ifaceState.IPv4)
		stats.IPv6AddressCount = len(ifaceState.IPv6)
		stats.LastCheck = time.Now()

		// Update interface statistics if available
		if ifaceState.Statistics != nil {
			stats.Status = ifaceState.Statistics.OperState
			stats.Speed = ifaceState.Statistics.Speed
			stats.RxBytes = ifaceState.Statistics.RxBytes
			stats.TxBytes = ifaceState.Statistics.TxBytes
			stats.RxBytesRate = ifaceState.Statistics.RxBytesRate
			stats.TxBytesRate = ifaceState.Statistics.TxBytesRate
			stats.RxPackets = ifaceState.Statistics.RxPackets
			stats.TxPackets = ifaceState.Statistics.TxPackets
			stats.RxPacketsRate = ifaceState.Statistics.RxPacketsRate
			stats.TxPacketsRate = ifaceState.Statistics.TxPacketsRate
			stats.Errors = int64(ifaceState.Statistics.RxErrors + ifaceState.Statistics.TxErrors)
			stats.Dropped = int64(ifaceState.Statistics.RxDropped + ifaceState.Statistics.TxDropped)
		}
	}

	// Remove metrics for interfaces that no longer exist
	for ifaceName := range m.NetworkStats {
		if _, exists := state.InterfaceInfo[ifaceName]; !exists {
			delete(m.NetworkStats, ifaceName)
		}
	}
}

// RecordInterfaceError records an interface-specific error
func (m *Metrics) RecordInterfaceError(ifaceName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stats, exists := m.NetworkStats[ifaceName]; exists {
		stats.ConsecutiveFailures++
		stats.LastError = err.Error()
		stats.LastErrorTime = time.Now()
	}
}

// RecordIPChange records IP address changes by interface
func (m *Metrics) RecordIPChange(oldState, newState *types.IPState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IPChanges.LastChangeTime = time.Now()
	m.IPChanges.TotalChanges++

	// Track changes per interface
	for ifaceName, newIfaceState := range newState.InterfaceInfo {
		// Get or create interface metrics
		if _, exists := m.NetworkStats[ifaceName]; !exists {
			m.NetworkStats[ifaceName] = &NetworkInterfaceMetrics{
				Name: ifaceName,
				Type: string(utils.GetInterfaceType(ifaceName)),
			}
		}

		stats := m.NetworkStats[ifaceName]
		oldIfaceState, existed := oldState.InterfaceInfo[ifaceName]

		if !existed {
			// New interface appeared
			stats.LastIPChange = time.Now()
			stats.TotalIPChanges++
			if len(newIfaceState.IPv4) > 0 {
				stats.IPv4Changes++
			}
			if len(newIfaceState.IPv6) > 0 {
				stats.IPv6Changes++
			}
			continue
		}

		// Compare IPv4 addresses
		if !utils.StringSlicesEqual(oldIfaceState.IPv4, newIfaceState.IPv4) {
			stats.IPv4Changes++
			stats.TotalIPChanges++
			stats.LastIPChange = time.Now()
			m.IPChanges.IPv4Changes++
		}

		// Compare IPv6 addresses
		if !utils.StringSlicesEqual(oldIfaceState.IPv6, newIfaceState.IPv6) {
			stats.IPv6Changes++
			stats.TotalIPChanges++
			stats.LastIPChange = time.Now()
			m.IPChanges.IPv6Changes++
		}
	}

	// Compare external IP if present
	if newState.ExternalIP != "" && newState.ExternalIP != oldState.ExternalIP {
		m.IPChanges.ExternalIPChanges++
	}

	// Calculate changes per day
	duration := time.Since(m.StartTime)
	days := duration.Hours() / 24
	if days > 0 {
		m.IPChanges.ChangesPerDay = float64(m.IPChanges.TotalChanges) / days
	}
}
