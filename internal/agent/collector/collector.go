package collector

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wameter/internal/agent/config"
	"wameter/internal/agent/reporter"
	"wameter/internal/notify"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Reporter defines interface for sending metrics
type Reporter = reporter.Reporter

// Notifier defines interface for sending notifications
type Notifier = notify.Notifier

// Collector defines the interface for all collectors
type Collector interface {
	// Start starts the collector
	Start(ctx context.Context) error
	// Stop stops the collector
	Stop() error
	// Name returns the collector name
	Name() string
	// Collect performs a single collection
	Collect(ctx context.Context) (*types.MetricsData, error)
}

// Manager manages multiple collectors
type Manager struct {
	collectors map[string]Collector
	config     *config.Config
	logger     *zap.Logger
	mu         sync.RWMutex
	startTime  time.Time
}

// NewManager creates a new collector manager
func NewManager(cfg *config.Config, logger *zap.Logger) *Manager {
	return &Manager{
		collectors: make(map[string]Collector),
		config:     cfg,
		logger:     logger,
		startTime:  time.Now(),
	}
}

// RegisterCollector registers a new collector
func (m *Manager) RegisterCollector(c Collector) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := c.Name()
	if _, exists := m.collectors[name]; exists {
		return fmt.Errorf("collector %s already registered", name)
	}

	m.collectors[name] = c
	m.logger.Info("Registered collector", zap.String("name", name))
	return nil
}

// Start starts all collectors
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, collector := range m.collectors {
		if err := collector.Start(ctx); err != nil {
			return fmt.Errorf("failed to start collector %s: %w", name, err)
		}
		m.logger.Info("Started collector", zap.String("name", name))
	}

	return nil
}

// Stop stops all collectors
func (m *Manager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error
	for name, collector := range m.collectors {
		if err := collector.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop collector %s: %w", name, err))
		}
		m.logger.Info("Stopped collector", zap.String("name", name))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping collectors: %v", errs)
	}
	return nil
}

// Collect runs all collectors and aggregates their results
func (m *Manager) Collect(ctx context.Context) (*types.MetricsData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &types.MetricsData{
		Timestamp:   time.Now(),
		CollectedAt: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errs := make(map[string]error)

	for name, collector := range m.collectors {
		wg.Add(1)
		go func(name string, c Collector) {
			defer wg.Done()

			data, err := c.Collect(ctx)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs[name] = err
				return
			}

			if data != nil {
				// Merge data into result
				if data.Metrics.Network != nil {
					result.Metrics.Network = data.Metrics.Network
				}
				// Add other metric types as needed
			}
		}(name, collector)
	}

	wg.Wait()

	if len(errs) > 0 {
		return result, fmt.Errorf("collection errors: %v", errs)
	}

	return result, nil
}

// StartTime returns the start time of the collector
func (m *Manager) StartTime() time.Time {
	return m.startTime
}
