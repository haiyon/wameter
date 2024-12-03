package storage

import (
	"fmt"
	"time"
)

// Config represents storage configuration
type Config struct {
	Driver          string        `mapstructure:"driver"`
	DSN             string        `mapstructure:"dsn"`
	MaxConnections  int           `mapstructure:"max_connections"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`

	// Migration settings
	AutoMigrate    bool   `mapstructure:"auto_migrate"`
	MigrationsPath string `mapstructure:"migrations_path"`

	// Data retention settings
	EnablePruning    bool          `mapstructure:"enable_pruning"`
	MetricsRetention time.Duration `mapstructure:"metrics_retention"`
	PruneInterval    time.Duration `mapstructure:"prune_interval"`

	// Query performance settings
	MaxBatchSize   int           `mapstructure:"max_batch_size"`
	MaxQueryRows   int           `mapstructure:"max_query_rows"`
	SlowQueryTime  time.Duration `mapstructure:"slow_query_time"`
	StatementCache bool          `mapstructure:"statement_cache"`

	// Metrics settings
	EnableMetrics bool `mapstructure:"enable_metrics"`
}

// Validate validates storage configuration
func (c *Config) Validate() error {
	if c.Driver == "" {
		return fmt.Errorf("storage driver is required")
	}
	if c.DSN == "" {
		return fmt.Errorf("storage DSN is required")
	}

	// Set default values
	if c.MaxConnections == 0 {
		c.MaxConnections = 25
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = time.Hour
	}
	if c.QueryTimeout == 0 {
		c.QueryTimeout = 30 * time.Second
	}
	if c.PruneInterval == 0 {
		c.PruneInterval = 24 * time.Hour
	}
	if c.MetricsRetention == 0 {
		c.MetricsRetention = 30 * 24 * time.Hour // 30 days
	}
	if c.MaxBatchSize == 0 {
		c.MaxBatchSize = 1000
	}
	if c.MaxQueryRows == 0 {
		c.MaxQueryRows = 10000
	}
	if c.SlowQueryTime == 0 {
		c.SlowQueryTime = time.Second
	}

	// Validate driver
	switch c.Driver {
	case "sqlite", "mysql", "postgres":
		// Valid drivers
	default:
		return fmt.Errorf("unsupported storage driver: %s", c.Driver)
	}

	return nil
}
