package types

import "time"

// AgentInfo represents agent information
type AgentInfo struct {
	ID           string      `json:"id"`
	Hostname     string      `json:"hostname"`
	Port         int         `json:"port"`
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

// AgentMetrics represents agent metrics
type AgentMetrics struct {
	CurrentStatus     string    `json:"current_status"`
	LastSeen          time.Time `json:"last_seen"`
	UptimePercent     float64   `json:"uptime_percent"`
	LastDowntime      time.Time `json:"last_downtime"`
	TotalCollections  int64     `json:"total_collections"`
	FailedCollections int64     `json:"failed_collections"`
	NetworkStats      struct {
		InterfaceCount int     `json:"interface_count"`
		TotalBandwidth uint64  `json:"total_bandwidth"`
		ErrorRate      float64 `json:"error_rate"`
	} `json:"network_stats"`
}
