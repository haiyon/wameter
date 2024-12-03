package types

import "time"

// AgentInfo represents agent information
type AgentInfo struct {
	ID           string      `json:"id"`
	Hostname     string      `json:"hostname"`
	Version      string      `json:"version"`
	Status       AgentStatus `json:"status"`
	LastSeen     time.Time   `json:"last_seen"`
	RegisteredAt time.Time   `json:"registered_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusOffline AgentStatus = "offline"
	AgentStatusError   AgentStatus = "error"
)

// IPState represents IP state
type IPState struct {
	IPv4          []string                   `json:"ipv4"`        // IPv4 addresses
	IPv6          []string                   `json:"ipv6"`        // IPv6 addresses
	ExternalIP    string                     `json:"external_ip"` // External IP address
	UpdatedAt     time.Time                  `json:"updated_at"`  // Last update time
	InterfaceInfo map[string]*InterfaceState `json:"interface_info"`
}

// InterfaceState represents the state of a network interface
type InterfaceState struct {
	Name       string          `json:"name"`
	MAC        string          `json:"mac"`
	MTU        int             `json:"mtu"`
	Flags      string          `json:"flags"`
	IPv4       []string        `json:"ipv4"`
	IPv6       []string        `json:"ipv6"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Statistics *InterfaceStats `json:"statistics,omitempty"`
}

// // InterfaceStats represents detailed network interface statistics
// type InterfaceStats struct {
// 	// Basic info
// 	IsUp       bool   `json:"is_up"`
// 	OperState  string `json:"oper_state"`
// 	Speed      int64  `json:"speed_mbps,omitempty"` // Interface speed in Mbps
// 	HasCarrier bool   `json:"has_carrier"`
// 	MTU        int    `json:"mtu"`
//
// 	// Traffic statistics
// 	RxBytes   uint64 `json:"rx_bytes"`
// 	TxBytes   uint64 `json:"tx_bytes"`
// 	RxPackets uint64 `json:"rx_packets"`
// 	TxPackets uint64 `json:"tx_packets"`
//
// 	// Error statistics
// 	RxErrors  uint64 `json:"rx_errors"`
// 	TxErrors  uint64 `json:"tx_errors"`
// 	RxDropped uint64 `json:"rx_dropped"`
// 	TxDropped uint64 `json:"tx_dropped"`
//
// 	// Detailed error statistics
// 	RxFifoErrors    uint64 `json:"rx_fifo_errors,omitempty"`
// 	TxFifoErrors    uint64 `json:"tx_fifo_errors,omitempty"`
// 	RxFrameErrors   uint64 `json:"rx_frame_errors,omitempty"`
// 	TxCarrierErrors uint64 `json:"tx_carrier_errors,omitempty"`
//
// 	// Additional statistics
// 	RxCompressed uint64 `json:"rx_compressed,omitempty"`
// 	TxCompressed uint64 `json:"tx_compressed,omitempty"`
// 	Multicast    uint64 `json:"multicast,omitempty"`
// 	Collisions   uint64 `json:"collisions,omitempty"`
//
// 	// Rate calculations (calculated fields)
// 	RxBytesRate   float64 `json:"rx_bytes_rate,omitempty"`   // bytes per second
// 	TxBytesRate   float64 `json:"tx_bytes_rate,omitempty"`   // bytes per second
// 	RxPacketsRate float64 `json:"rx_packets_rate,omitempty"` // packets per second
// 	TxPacketsRate float64 `json:"tx_packets_rate,omitempty"` // packets per second
//
// 	// Timestamp
// 	CollectedAt time.Time `json:"collected_at"`
// }
