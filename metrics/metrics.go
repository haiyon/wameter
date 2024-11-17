package metrics

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/haiyon/ip-monitor/types"
	"github.com/haiyon/ip-monitor/utils"
)

// Metrics represents monitoring metrics
type Metrics struct {
	mu            sync.RWMutex
	StartTime     time.Time        `json:"start_time"`
	LastCheckTime time.Time        `json:"last_check_time"`
	CheckCount    int64            `json:"check_count"`
	ErrorCount    int64            `json:"error_count"`
	LastError     string           `json:"last_error"`
	IPChanges     *IPChangeMetrics `json:"ip_changes"`
	ProviderStats *ProviderMetrics `json:"provider_stats"`
	NetworkStats  *NetworkMetrics  `json:"network_stats"`
}

// IPChangeMetrics tracks IP address changes
type IPChangeMetrics struct {
	LastChangeTime    time.Time `json:"last_change_time"`
	TotalChanges      int64     `json:"total_changes"`
	IPv4Changes       int64     `json:"ipv4_changes"`
	IPv6Changes       int64     `json:"ipv6_changes"`
	ExternalIPChanges int64     `json:"external_ip_changes"`
	ChangesPerDay     float64   `json:"changes_per_day"`
}

// ProviderMetrics tracks external IP provider performance
type ProviderMetrics struct {
	IPv4Providers map[string]*ProviderStats `json:"ipv4_providers"`
	IPv6Providers map[string]*ProviderStats `json:"ipv6_providers"`
}

// ProviderStats represents statistics for a single provider
type ProviderStats struct {
	Requests            int64         `json:"requests"`
	Successes           int64         `json:"successes"`
	Failures            int64         `json:"failures"`
	LastResponseTime    time.Duration `json:"last_response_time"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastSuccess         time.Time     `json:"last_success"`
	LastError           string        `json:"last_error"`
}

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

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime: time.Now(),
		IPChanges: &IPChangeMetrics{},
		ProviderStats: &ProviderMetrics{
			IPv4Providers: make(map[string]*ProviderStats),
			IPv6Providers: make(map[string]*ProviderStats),
		},
		NetworkStats: &NetworkMetrics{},
	}
}

// RecordCheck records a check attempt
func (m *Metrics) RecordCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CheckCount++
	m.LastCheckTime = time.Now()
}

// RecordError records an error
func (m *Metrics) RecordError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ErrorCount++
	m.LastError = err.Error()
}

// RecordProviderRequest records metrics for an external IP provider request
func (m *Metrics) RecordProviderRequest(ipVersion string, provider string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var stats *ProviderStats
	if ipVersion == "ipv4" {
		if m.ProviderStats.IPv4Providers == nil {
			m.ProviderStats.IPv4Providers = make(map[string]*ProviderStats)
		}
		if m.ProviderStats.IPv4Providers[provider] == nil {
			m.ProviderStats.IPv4Providers[provider] = &ProviderStats{}
		}
		stats = m.ProviderStats.IPv4Providers[provider]
	} else {
		if m.ProviderStats.IPv6Providers == nil {
			m.ProviderStats.IPv6Providers = make(map[string]*ProviderStats)
		}
		if m.ProviderStats.IPv6Providers[provider] == nil {
			m.ProviderStats.IPv6Providers[provider] = &ProviderStats{}
		}
		stats = m.ProviderStats.IPv6Providers[provider]
	}

	stats.Requests++
	stats.LastResponseTime = duration

	if err != nil {
		stats.Failures++
		stats.LastError = err.Error()
	} else {
		stats.Successes++
		stats.LastSuccess = time.Now()
	}

	// Update average response time
	totalTime := stats.AverageResponseTime.Nanoseconds() * (stats.Requests - 1)
	totalTime += duration.Nanoseconds()
	stats.AverageResponseTime = time.Duration(totalTime / stats.Requests)
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

// GetSnapshot returns a copy of current metrics
func (m *Metrics) GetSnapshot() (*Metrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var snapshot Metrics
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}
