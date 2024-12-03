package types

import (
	"encoding/json"
	"time"

	"wameter/internal/validator"
)

var validate = validator.New()

// NetworkState represents the current state of network interfaces
type NetworkState struct {
	AgentID     string                    `json:"agent_id" validate:"required"`
	Hostname    string                    `json:"hostname" validate:"required"`
	Timestamp   time.Time                 `json:"timestamp" validate:"required"`
	Interfaces  map[string]*InterfaceInfo `json:"interfaces" validate:"required,dive"`
	ExternalIP  string                    `json:"external_ip,omitempty" validate:"omitempty,ip"`
	CollectedAt time.Time                 `json:"collected_at" validate:"required"`
	ReportedAt  time.Time                 `json:"reported_at" validate:"required"`
}

// Validate performs validation of NetworkState
func (n *NetworkState) Validate() error {
	return validate.Struct(n)
}

// MergeStats merges interface statistics
func (n *NetworkState) MergeStats(stats map[string]*InterfaceStats) {
	for name, iface := range n.Interfaces {
		if stat, ok := stats[name]; ok {
			iface.Statistics = stat
		}
	}
}

// InterfaceInfo represents detailed information about a network interface
type InterfaceInfo struct {
	Name       string          `json:"name" validate:"required"`
	Type       string          `json:"type" validate:"required"`
	MAC        string          `json:"mac" validate:"required,mac"`
	MTU        int             `json:"mtu" validate:"required,min=1"`
	Flags      string          `json:"flags"`
	IPv4       []string        `json:"ipv4" validate:"dive,ip"`
	IPv6       []string        `json:"ipv6" validate:"dive,ip"`
	Status     string          `json:"status"`
	Statistics *InterfaceStats `json:"statistics,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at" validate:"required"`
}

// Validate performs validation of InterfaceInfo
func (i *InterfaceInfo) Validate() error {
	return validate.Struct(i)
}

// IsPhysical checks if the interface is physical
func (i *InterfaceInfo) IsPhysical() bool {
	return i.Type == "ethernet" || i.Type == "wireless"
}

// GetPrimaryIP returns primary IP address
func (i *InterfaceInfo) GetPrimaryIP() string {
	if len(i.IPv4) > 0 {
		return i.IPv4[0]
	}
	if len(i.IPv6) > 0 {
		return i.IPv6[0]
	}
	return ""
}

// InterfaceStats represents network interface statistics
type InterfaceStats struct {
	// Basic info
	IsUp       bool   `json:"is_up"`
	OperState  string `json:"oper_state"`
	Speed      int64  `json:"speed_mbps,omitempty"`
	HasCarrier bool   `json:"has_carrier"`

	// Traffic statistics
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	TxErrors  uint64 `json:"tx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxDropped uint64 `json:"tx_dropped"`

	// Rate calculations
	RxBytesRate   float64 `json:"rx_bytes_rate"`
	TxBytesRate   float64 `json:"tx_bytes_rate"`
	RxPacketsRate float64 `json:"rx_packets_rate"`
	TxPacketsRate float64 `json:"tx_packets_rate"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// MetricsData represents collected metrics data
type MetricsData struct {
	AgentID     string    `json:"agent_id"`
	Hostname    string    `json:"hostname"`
	Timestamp   time.Time `json:"timestamp"`
	CollectedAt time.Time `json:"collected_at"`
	ReportedAt  time.Time `json:"reported_at"`
	Metrics     struct {
		Network *NetworkState `json:"network,omitempty"`
	} `json:"metrics"`
}

// ToJSON converts MetricsData to JSON
func (m *MetricsData) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON converts JSON to MetricsData
func (m *MetricsData) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}
