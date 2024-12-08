package types

import "time"

// MetricsSummary represents a summary of metrics data
type MetricsSummary struct {
	CurrentStatus  string    `json:"current_status"`
	TotalMetrics   int64     `json:"total_metrics"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	NetworkMetrics struct {
		TotalTraffic   uint64  `json:"total_traffic"`
		AvgUtilization float64 `json:"avg_utilization"`
		ErrorRate      float64 `json:"error_rate"`
		IPChanges      int64   `json:"ip_changes"`
	} `json:"network_metrics"`
}

// MetricsFilter represents metrics query filter options
type MetricsFilter struct {
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	AgentIDs    []string  `json:"agent_ids,omitempty"`
	MetricTypes []string  `json:"metric_types,omitempty"`
	Status      []string  `json:"status,omitempty"`
	SortBy      string    `json:"sort_by,omitempty"`
	SortOrder   string    `json:"sort_order,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int       `json:"offset,omitempty"`
}

// MetricsQuery represents a metrics query with pagination
type MetricsQuery struct {
	Filter     MetricsFilter `json:"filter"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
	Total      int64         `json:"total"`
}

// MetricsExport represents metrics export options
type MetricsExport struct {
	Format     string        `json:"format"`
	Filter     MetricsFilter `json:"filter"`
	Compress   bool          `json:"compress"`
	IncludeRaw bool          `json:"include_raw"`
}

// MetricsArchiveOptions represents metrics archiving options
type MetricsArchiveOptions struct {
	Before      time.Time `json:"before"`
	StorageType string    `json:"storage_type"`
	Compress    bool      `json:"compress"`
	DeleteAfter bool      `json:"delete_after"`
}
