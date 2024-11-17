package types

import "time"

// IPState represents IP state
type IPState struct {
	IPv4       []string  `json:"ipv4"`        // IPv4 addresses
	IPv6       []string  `json:"ipv6"`        // IPv6 addresses
	ExternalIP string    `json:"external_ip"` // External IP address
	UpdatedAt  time.Time `json:"updated_at"`  // Last update time
}
