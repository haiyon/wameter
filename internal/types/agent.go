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
