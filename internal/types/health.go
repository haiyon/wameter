package types

import (
	"runtime"
	"time"
)

// HealthStatus represents the overall system health status
type HealthStatus struct {
	Healthy   bool              `json:"healthy"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	StartTime time.Time         `json:"start_time"`
	Uptime    time.Duration     `json:"uptime"`
	Details   []ComponentStatus `json:"details,omitempty"`
}

// ComponentStatus represents individual component status
type ComponentStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Error     string    `json:"error,omitempty"`
	LastCheck time.Time `json:"last_check"`
}

// ServiceMetrics represents comprehensive service metrics
type ServiceMetrics struct {
	StartTime        time.Time     `json:"start_time"`
	SystemInfo       *SystemStats  `json:"system_info"`
	DatabaseStats    DatabaseStats `json:"database_stats"`
	ActiveAgents     int           `json:"active_agents"`
	TotalAgents      int           `json:"total_agents"`
	MetricsProcessed int64         `json:"metrics_processed"`
	IPChanges        int64         `json:"ip_changes"`
	Notifications    int64         `json:"notifications"`
	ErrorCount       int64         `json:"error_count"`
	LastError        string        `json:"last_error,omitempty"`
	LastErrorTime    time.Time     `json:"last_error_time,omitempty"`
}

// SystemStats represents system statistics
type SystemStats struct {
	NumGoroutine int              `json:"num_goroutine"`
	MemoryAlloc  uint64           `json:"memory_alloc"`
	MemoryTotal  uint64           `json:"memory_total"`
	MemStats     runtime.MemStats `json:"mem_stats"`
	NumGC        uint32           `json:"num_gc"`
	LastGC       time.Time        `json:"last_gc"`
	CPUUsage     float64          `json:"cpu_usage"`
	DiskUsage    float64          `json:"disk_usage"`
}

// DatabaseStats represents database statistics
type DatabaseStats struct {
	OpenConnections int           `json:"open_connections"`
	InUse           int           `json:"in_use"`
	Idle            int           `json:"idle"`
	WaitCount       int64         `json:"wait_count"`
	QueryCount      int64         `json:"query_count"`
	ErrorCount      int64         `json:"error_count"`
	SlowQueries     int64         `json:"slow_queries"`
	AvgQueryTime    time.Duration `json:"avg_query_time"`
}
