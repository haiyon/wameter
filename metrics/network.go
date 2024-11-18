package metrics

import (
	"time"

	"ip-monitor/types"
	"ip-monitor/utils"
)

// NetworkMetrics tracks network interface metrics
type NetworkMetrics struct {
	IPv4AddressCount    int       `json:"ipv4_address_count"`
	IPv6AddressCount    int       `json:"ipv6_address_count"`
	InterfaceStatus     string    `json:"interface_status"`
	LastInterfaceCheck  time.Time `json:"last_interface_check"`
	ConsecutiveFailures int64     `json:"consecutive_failures"`
	UptimeSeconds       int64     `json:"uptime_seconds"`
	InterfaceSpeed      int64     `json:"interface_speed"`
	InterfaceErrors     int64     `json:"interface_errors"`
}

// RecordIPChange records IP address changes
func (m *Metrics) RecordIPChange(oldState, newState *types.IPState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IPChanges.LastChangeTime = time.Now()
	m.IPChanges.TotalChanges++

	// Compare IPv4 addresses
	if !utils.StringSlicesEqual(oldState.IPv4, newState.IPv4) {
		m.IPChanges.IPv4Changes++
	}

	// Compare IPv6 addresses
	if !utils.StringSlicesEqual(oldState.IPv6, newState.IPv6) {
		m.IPChanges.IPv6Changes++
	}

	// Compare external IP
	if oldState.ExternalIP != newState.ExternalIP {
		m.IPChanges.ExternalIPChanges++
	}

	// Calculate changes per day
	duration := time.Since(m.StartTime)
	days := duration.Hours() / 24
	if days > 0 {
		m.IPChanges.ChangesPerDay = float64(m.IPChanges.TotalChanges) / days
	}
}

// UpdateNetworkStats updates network interface metrics
func (m *Metrics) UpdateNetworkStats(state *types.IPState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NetworkStats.IPv4AddressCount = len(state.IPv4)
	m.NetworkStats.IPv6AddressCount = len(state.IPv6)
	m.NetworkStats.LastInterfaceCheck = time.Now()
}
