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
