package collector

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wameter/internal/agent/collector/network"
	"wameter/internal/agent/config"
	"wameter/internal/agent/notify"
	"wameter/internal/agent/reporter"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Manager manages multiple collectors
type Manager struct {
	reporter   *reporter.Reporter
	notifier   *notify.Manager
	collectors map[string]Collector
	config     *config.Config
	logger     *zap.Logger
	mu         sync.RWMutex
	startTime  time.Time
}

// NewManager creates new collector manager
func NewManager(cfg *config.Config, reporter *reporter.Reporter, notifier *notify.Manager, logger *zap.Logger) *Manager {
	return &Manager{
		reporter:   reporter,
		notifier:   notifier,
		collectors: make(map[string]Collector),
		config:     cfg,
		logger:     logger,
		startTime:  time.Now(),
	}
}

// RegisterCollector registers new collector
func (m *Manager) RegisterCollector(c Collector) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := c.Name()
	if _, exists := m.collectors[name]; exists {
		return fmt.Errorf("collector %s already registered", name)
	}

	m.collectors[name] = c
	return nil
}

// Start starts all collectors
func (m *Manager) Start(ctx context.Context) error {
	// Initialize all collectors
	if err := m.initCollectors(); err != nil {
		return fmt.Errorf("failed to initialize collectors: %w", err)
	}

	// Start all collectors
	for name, collector := range m.collectors {
		m.mu.RLock()
		err := collector.Start(ctx)
		m.mu.RUnlock()
		if err != nil {
			return fmt.Errorf("failed to start collector %s: %w", name, err)
		}
		m.logger.Info("Collector started", zap.String("name", name))
	}

	// Start collection loop
	go m.startCollectorLoop(ctx)

	return nil
}

// Stop stops all collectors
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, collector := range m.collectors {
		if err := collector.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop collector %s: %w", name, err))
		}
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

	// Launch collectors
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

// GetReporter returns the current reporter
func (m *Manager) GetReporter() *reporter.Reporter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reporter
}

// initCollectors initializes all configured collectors
func (m *Manager) initCollectors() error {
	// Initialize network collector if enabled
	if m.config.Collector.Network.Enabled {
		networkCollector := network.NewCollector(
			&m.config.Collector.Network,
			m.config.Agent.ID,
			m.reporter,
			m.notifier,
			m.config.Agent.Standalone,
			m.logger,
		)
		if err := m.RegisterCollector(networkCollector); err != nil {
			return fmt.Errorf("failed to register network collector: %w", err)
		}
	}

	// Add other collectors as needed

	return nil
}

// startCollectorLoop starts the collector loop
func (m *Manager) startCollectorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.Collector.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, err := m.Collect(ctx)
			if err != nil {
				m.logger.Error("Failed to collect metrics", zap.Error(err))
				continue
			}

			if data == nil {
				m.logger.Debug("No data collected")
				continue
			}

			// Ensure we have basic data fields
			if data.Hostname == "" {
				data.Hostname = m.config.Agent.Hostname
			}

			data.ReportedAt = time.Now()

			// Send data if we have any
			if !m.config.Agent.Standalone && m.reporter != nil {
				if err := m.reporter.Report(data); err != nil {
					m.logger.Error("Failed to report metrics", zap.Error(err))
				}
			}
		}
	}
}
