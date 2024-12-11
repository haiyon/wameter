package collector

import (
	"context"
	"wameter/internal/types"
)

// Collector defines the interface for all collectors
type Collector interface {
	// Name returns the collector name
	Name() string
	// Start starts the collector
	Start(ctx context.Context) error
	// Collect performs a single collection
	Collect(ctx context.Context) (*types.MetricsData, error)
	// Stop stops the collector
	Stop() error
}
