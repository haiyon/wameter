package types

import "time"

// IPChangeSummary represents a summary of IP changes
type IPChangeSummary struct {
	CurrentStatus      string    `json:"current_status"`
	LastCheck          time.Time `json:"last_check"`
	TotalChanges       int64     `json:"total_changes"`
	AffectedInterfaces int       `json:"affected_interfaces"`
	ExternalChanges    int64     `json:"external_changes"`
	FirstChange        time.Time `json:"first_change"`
	LastChange         time.Time `json:"last_change"`
	AvgDailyChanges    float64   `json:"avg_daily_changes"`
	MaxDailyChanges    int       `json:"max_daily_changes"`
	ChangesByVersion   struct {
		IPv4Changes int64 `json:"ipv4_changes"`
		IPv6Changes int64 `json:"ipv6_changes"`
	} `json:"changes_by_version"`
	ChangesByAction struct {
		Added   int64 `json:"added"`
		Updated int64 `json:"updated"`
		Removed int64 `json:"removed"`
	} `json:"changes_by_action"`
}

// IPChangeFilter represents filtering options for IP changes
type IPChangeFilter struct {
	StartTime  time.Time   `json:"start_time"`
	EndTime    time.Time   `json:"end_time"`
	Interfaces []string    `json:"interfaces,omitempty"`
	Versions   []IPVersion `json:"versions,omitempty"`
	IsExternal *bool       `json:"is_external,omitempty"`
	Actions    []string    `json:"actions,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
}

// IPChangeStats represents IP change statistics
type IPChangeStats struct {
	TotalChanges    int64   `json:"total_changes"`
	ChangesPerDay   float64 `json:"changes_per_day"`
	ChangesPerWeek  float64 `json:"changes_per_week"`
	ChangesPerMonth float64 `json:"changes_per_month"`
	MostActiveHour  int     `json:"most_active_hour"`
	MostActiveDay   int     `json:"most_active_day"`
	AverageInterval float64 `json:"average_interval"` // in hours
}
