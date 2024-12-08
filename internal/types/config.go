package types

import "time"

// ConfigChange represents a configuration change record
type ConfigChange struct {
	Timestamp time.Time            `json:"timestamp"`
	User      string               `json:"user,omitempty"`
	Changes   []ConfigModification `json:"changes"`
}

// ConfigModification represents a single configuration modification
type ConfigModification struct {
	Path     string `json:"path"`
	OldValue any    `json:"old_value,omitempty"`
	NewValue any    `json:"new_value"`
}
