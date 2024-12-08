package types

import (
	"encoding/json"
	"time"
)

// Command represents a command to be sent to an agent
type Command struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Data      any           `json:"data,omitempty"`
	Timeout   time.Duration `json:"timeout,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	CommandID string          `json:"command_id"`
	AgentID   string          `json:"agent_id"`
	Status    CommandStatus   `json:"status"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time,omitempty"`
}

// CommandHistory represents a historical command record
type CommandHistory struct {
	Command  Command       `json:"command"`
	Result   CommandResult `json:"result"`
	Duration time.Duration `json:"duration"`
}

// CommandStatus represents command execution status
type CommandStatus string

const (
	CommandStatusPending  CommandStatus = "pending"
	CommandStatusRunning  CommandStatus = "running"
	CommandStatusComplete CommandStatus = "complete"
	CommandStatusFailed   CommandStatus = "failed"
	CommandStatusCanceled CommandStatus = "canceled"
	CommandStatusTimedOut CommandStatus = "timed_out"
)
