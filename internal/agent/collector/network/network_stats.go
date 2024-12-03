package network

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"time"

	"wameter/internal/agent/config"
	"wameter/internal/types"
	"wameter/internal/utils"

	"go.uber.org/zap"
)

// statsCollector represents a stats collector
type statsCollector struct {
	config    *config.NetworkConfig
	logger    *zap.Logger
	stats     map[string]*types.InterfaceStats
	prevStats map[string]*types.InterfaceStats
	mu        sync.RWMutex
	stopChan  chan struct{}
}

// newStatsCollector creates a new stats collector
func newStatsCollector(cfg *config.NetworkConfig, logger *zap.Logger) *statsCollector {
	return &statsCollector{
		config:    cfg,
		logger:    logger,
		stats:     make(map[string]*types.InterfaceStats),
		prevStats: make(map[string]*types.InterfaceStats),
		stopChan:  make(chan struct{}),
	}
}

// Start starts the stats collector
func (s *statsCollector) Start(ctx context.Context) error {
	// Collect initial stats
	if err := s.collect(); err != nil {
		s.logger.Warn("Failed to collect initial stats", zap.Error(err))
	}

	// Start collection loop
	go func() {
		ticker := time.NewTicker(s.config.StatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.collect(); err != nil {
					s.logger.Error("Failed to collect stats", zap.Error(err))
				}
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			}
		}
	}()

	return nil
}

// Stop stops the stats collector
func (s *statsCollector) Stop() error {
	close(s.stopChan)
	return nil
}

// GetStats returns the current stats
func (s *statsCollector) GetStats() map[string]*types.InterfaceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Make a copy of the stats
	stats := make(map[string]*types.InterfaceStats)
	for iface, stat := range s.stats {
		statCopy := *stat
		stats[iface] = &statCopy
	}

	return stats
}

// collect collects network statistics
func (s *statsCollector) collect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save current stats as previous
	s.prevStats = s.stats
	s.stats = make(map[string]*types.InterfaceStats)

	// Get all interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, iface := range interfaces {
		// Skip interfaces based on configuration
		if !shouldMonitorInterface(iface.Name, iface.Flags, s.config) {
			continue
		}

		stats, err := utils.GetInterfaceStats(iface.Name)
		if err != nil {
			s.logger.Debug("Failed to get interface stats",
				zap.String("interface", iface.Name),
				zap.Error(err))
			continue
		}

		// Calculate rates if we have previous stats
		if prevStats, exists := s.prevStats[iface.Name]; exists {
			duration := stats.CollectedAt.Sub(prevStats.CollectedAt).Seconds()
			if duration > 0 {
				stats.RxBytesRate = float64(stats.RxBytes-prevStats.RxBytes) / duration
				stats.TxBytesRate = float64(stats.TxBytes-prevStats.TxBytes) / duration
				stats.RxPacketsRate = float64(stats.RxPackets-prevStats.RxPackets) / duration
				stats.TxPacketsRate = float64(stats.TxPackets-prevStats.TxPackets) / duration
			}
		}

		s.stats[iface.Name] = stats
	}

	return nil
}

// Helper function to check if interface should be monitored
func shouldMonitorInterface(name string, flags net.Flags, config *config.NetworkConfig) bool {
	// Check exclusion patterns
	for _, pattern := range config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return false
		}
	}

	// Skip virtual interfaces unless explicitly enabled
	if !config.IncludeVirtual && utils.IsVirtualInterface(name) {
		return false
	}

	// Check if interface is up
	if flags&net.FlagUp == 0 {
		return false
	}

	return true
}
