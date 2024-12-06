package database

import (
	"context"
	"fmt"
	"time"
	"wameter/internal/server/config"

	"wameter/internal/types"

	"go.uber.org/zap"
)

// Database defines the interface for metric database
type Database interface {
	// Metrics

	SaveMetrics(ctx context.Context, data *types.MetricsData) error
	BatchSaveMetrics(ctx context.Context, metrics []*types.MetricsData) error
	GetMetrics(ctx context.Context, query *MetricsQuery, opts QueryOptions) ([]*types.MetricsData, error)
	GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error)
	SaveIPChange(ctx context.Context, agentID string, change *types.IPChange) error

	// Agent

	RegisterAgent(ctx context.Context, agent *types.AgentInfo) error
	UpdateAgentStatus(ctx context.Context, agentID string, status types.AgentStatus) error
	GetAgents(ctx context.Context) ([]*types.AgentInfo, error)
	GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error)

	// Command

	// StartPruning starts the background pruning routine
	StartPruning(ctx context.Context) error
	// StopPruning stops the pruning routine
	StopPruning() error

	// Maintenance

	Cleanup(ctx context.Context, before time.Time) error
	Stats() Stats

	// Stats / Health

	Ping(ctx context.Context) error
	Close() error
}

// Metrics represents database metrics
type Metrics struct {
	QueryCount     int64
	QueryErrors    int64
	SlowQueryCount int64
	LastError      error
	LastErrorTime  time.Time
}

// NewDatabase creates a database instance based on driver type
func NewDatabase(config *config.Database, logger *zap.Logger) (Database, error) {
	// Default options
	opts := Options{
		MaxOpenConns:     25,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnMaxIdleTime:  30 * time.Minute,
		QueryTimeout:     30 * time.Second,
		EnablePruning:    false,
		MetricsRetention: 30 * 24 * time.Hour,
		PruneInterval:    24 * time.Hour,
	}

	// Override with config if provided
	if config.MaxConnections > 0 {
		opts.MaxOpenConns = config.MaxConnections
	}
	if config.MaxIdleConns > 0 {
		opts.MaxIdleConns = config.MaxIdleConns
	}
	if config.ConnMaxLifetime > 0 {
		opts.ConnMaxLifetime = config.ConnMaxLifetime
	}
	if config.QueryTimeout > 0 {
		opts.QueryTimeout = config.QueryTimeout
	}
	if config.EnablePruning {
		opts.EnablePruning = config.EnablePruning
	}
	if config.MetricsRetention > 0 {
		opts.MetricsRetention = config.MetricsRetention
	}
	if config.PruneInterval > 0 {
		opts.PruneInterval = config.PruneInterval
	}

	// Create database based on driver
	switch config.Driver {
	case "sqlite":
		return NewSQLiteDatabase(config.DSN, opts, logger)
	case "mysql":
		return NewMySQLDatabase(config.DSN, opts, logger)
	case "postgres":
		return NewPostgresDatabase(config.DSN, opts, logger)
	default:
		return nil, fmt.Errorf("%w: %s", types.ErrInvalidDriver, config.Driver)
	}
}

// Stats represents database database statistics
type Stats struct {
	// Connection pool stats
	OpenConnections   int           `json:"open_connections"`
	InUse             int           `json:"in_use"`
	Idle              int           `json:"idle"`
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration"`
	MaxIdleClosed     int64         `json:"max_idle_closed"`
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"`

	// Query stats
	QueryCount  int64 `json:"query_count"`
	QueryErrors int64 `json:"query_errors"`
	SlowQueries int64 `json:"slow_queries"`

	// Size stats
	DatabaseSize int64            `json:"database_size"`
	TableSizes   map[string]int64 `json:"table_sizes"`

	// Performance stats
	AvgQueryTime time.Duration    `json:"avg_query_time"`
	MaxQueryTime time.Duration    `json:"max_query_time"`
	IndexUsage   map[string]int64 `json:"index_usage"`

	// Cache stats
	CacheHits   int64 `json:"cache_hits"`
	CacheMisses int64 `json:"cache_misses"`
	CacheSize   int64 `json:"cache_size"`
}

// String returns string representation of stats
func (s Stats) String() string {
	return fmt.Sprintf(
		"Connections: %d (in-use: %d, idle: %d), "+
			"Queries: %d (errors: %d, slow: %d), "+
			"Wait count: %d, Wait duration: %v",
		s.OpenConnections, s.InUse, s.Idle,
		s.QueryCount, s.QueryErrors, s.SlowQueries,
		s.WaitCount, s.WaitDuration,
	)
}
