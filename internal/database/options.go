package database

import "time"

// Options defines database options
type Options struct {
	// Connection settings
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"`

	// Query settings
	QueryTimeout   time.Duration `json:"query_timeout"`
	MaxBatchSize   int           `json:"max_batch_size"`
	StatementCache bool          `json:"statement_cache"`

	// Metrics settings
	EnableMetrics      bool          `json:"enable_metrics"`
	SlowQueryThreshold time.Duration `json:"slow_query_threshold"`

	// Data pruning settings
	EnablePruning   bool          `json:"enable_pruning"`
	PruneInterval   time.Duration `json:"prune_interval"`
	RetentionPeriod time.Duration `json:"retention_period"`
}

// Stats represents database statistics
type Stats struct {
	// Connection stats
	OpenConnections int           `json:"open_connections"`
	InUse           int           `json:"in_use"`
	Idle            int           `json:"idle"`
	WaitCount       int64         `json:"wait_count"`
	WaitDuration    time.Duration `json:"wait_duration"`

	// Query stats
	QueryCount   int64         `json:"query_count"`
	QueryErrors  int64         `json:"query_errors"`
	SlowQueries  int64         `json:"slow_queries"`
	AvgQueryTime time.Duration `json:"avg_query_time"`

	// Cache stats
	CacheHits   int64 `json:"cache_hits"`
	CacheMisses int64 `json:"cache_misses"`
}
