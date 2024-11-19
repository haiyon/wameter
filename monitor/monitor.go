package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"ip-monitor/config"
	"ip-monitor/metrics"
	"ip-monitor/notifier"
	"ip-monitor/types"

	"go.uber.org/zap"
)

// Monitor handles IP monitoring
type Monitor struct {
	config         *config.Config
	logger         *zap.Logger
	metrics        *metrics.Metrics
	notifier       *notifier.Notifier
	lastState      types.IPState
	mu             sync.RWMutex
	client         *http.Client
	ctx            context.Context
	cancel         context.CancelFunc
	statsCollector *NetworkStatsCollector
}

// NewMonitor creates a new Monitor instance
func NewMonitor(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Monitor, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Initialize HTTP client with timeouts and connection pooling
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	// Initialize notifier
	n, err := notifier.NewNotifier(cfg, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize notifier: %w", err)
	}

	m := &Monitor{
		config:   cfg,
		logger:   logger,
		metrics:  metrics.NewMetrics(),
		notifier: n,
		client:   client,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Initialize network stats collector
	m.statsCollector = NewNetworkStatsCollector(ctx, cfg.InterfaceConfig, logger, m.metrics)

	// Load last known state
	if err := m.loadState(); err != nil {
		logger.Warn("Failed to load last state", zap.Error(err))
	}

	return m, nil
}

// Start begins the monitoring process
func (m *Monitor) Start() error {
	m.logger.Info("Starting IP monitor",
		zap.Bool("external_ip_enabled", m.config.CheckExternalIP),
		zap.Any("interface_types", m.config.InterfaceConfig.InterfaceTypes),
		zap.Bool("include_virtual", m.config.InterfaceConfig.IncludeVirtual))

	// Start network stats collector
	go func() {
		if err := m.statsCollector.Start(); err != nil {
			m.logger.Error("Failed to start network stats collector", zap.Error(err))
		}
	}()

	// Perform initial check and send notification
	if err := m.initialCheck(); err != nil {
		m.logger.Error("Initial IP check failed", zap.Error(err))
	}

	// Create ticker for regular checks
	ticker := time.NewTicker(time.Duration(m.config.CheckInterval) * time.Second)
	defer ticker.Stop()

	// Main monitoring loop
	for {
		select {
		case <-ticker.C:
			if err := m.checkIP(m.ctx); err != nil {
				m.logger.Error("IP check failed", zap.Error(err))
			}
		case <-m.ctx.Done():
			return nil
		}
	}
}

// Stop gracefully stops the monitor
func (m *Monitor) Stop(ctx context.Context) error {
	m.logger.Info("Stopping IP monitor...")

	// Stop network stats collector
	m.statsCollector.Stop()

	m.cancel()

	done := make(chan error, 1)
	go func() {
		// Save final state
		if err := m.saveState(); err != nil {
			m.logger.Error("Failed to save final state", zap.Error(err))
			done <- err
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
}

// checkIP performs a single IP check iteration
func (m *Monitor) checkIP(ctx context.Context) error {
	start := time.Now()
	defer func() {
		m.metrics.RecordCheck()
		duration := time.Since(start)
		m.logger.Debug("IP check completed", zap.Duration("duration", duration))
	}()

	// Create current state
	var currentState *types.IPState

	// Get internal IPs with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		currentState, err = m.getCurrentIPs()
	}()

	// Wait for IP check or timeout
	select {
	case <-checkCtx.Done():
		return fmt.Errorf("IP check timed out after 30s: %w", checkCtx.Err())
	case <-done:
		if err != nil {
			m.metrics.RecordError(err)
			return fmt.Errorf("failed to get IPs: %w", err)
		}
	}

	// Update network stats
	m.metrics.UpdateNetworkStats(currentState)

	// Get external IP if enabled
	if m.config.CheckExternalIP {
		externalIP, err := m.getExternalIP(ctx)
		if err != nil {
			m.logger.Warn("Failed to get external IP", zap.Error(err))
		} else {
			currentState.ExternalIP = externalIP
			m.logger.Debug("Got external IP", zap.String("ip", externalIP))
		}
	}

	// Check for changes and handle them
	if changed, changes := m.hasIPChanged(currentState); changed {
		m.logger.Info("IP changes detected", zap.Strings("changes", changes))

		if err := m.handleIPChange(*currentState, changes); err != nil {
			m.metrics.RecordError(err)
			return fmt.Errorf("failed to handle IP change: %w", err)
		}

		// Record metrics for IP changes
		m.metrics.RecordIPChange(&m.lastState, currentState)

		// Save state after successful change handling
		if err := m.saveState(); err != nil {
			m.logger.Error("Failed to save state", zap.Error(err))
		}
	}

	return nil
}

// handleIPChange handles IP address changes
func (m *Monitor) handleIPChange(newState types.IPState, changes []string) error {
	// Log changes
	m.logger.Info("IP address changed",
		zap.Time("time", newState.UpdatedAt),
		zap.Int("interface_count", len(newState.InterfaceInfo)),
		zap.Strings("changes", changes))

	// Send notifications
	if err := m.notifier.NotifyIPChange(m.lastState, newState, changes); err != nil {
		return fmt.Errorf("failed to send notifications: %w", err)
	}

	// Update state
	m.mu.Lock()
	m.lastState = newState
	m.mu.Unlock()

	return nil
}

// loadState loads the last known state from file
func (m *Monitor) loadState() error {
	data, err := os.ReadFile(m.config.LastIPFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state types.IPState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	m.mu.Lock()
	m.lastState = state
	m.mu.Unlock()

	return nil
}

// saveState saves the current state to file
func (m *Monitor) saveState() error {
	m.mu.RLock()
	data, err := json.Marshal(m.lastState)
	m.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temporary file first
	tmpFile := m.config.LastIPFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	// Rename temporary file to actual file (atomic operation)
	if err := os.Rename(tmpFile, m.config.LastIPFile); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	return nil
}
