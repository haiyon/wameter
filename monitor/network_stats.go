package monitor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/haiyon/wameter/config"
	"github.com/haiyon/wameter/metrics"

	"github.com/haiyon/wameter/types"
	"github.com/haiyon/wameter/utils"

	"go.uber.org/zap"
)

// NetworkStatsCollector handles collection of network interface statistics
type NetworkStatsCollector struct {
	config  *config.InterfaceConfig
	logger  *zap.Logger
	metrics *metrics.Metrics
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	stats   map[string]*types.InterfaceStats
}

// NewNetworkStatsCollector creates a new network statistics collector
func NewNetworkStatsCollector(ctx context.Context, config *config.InterfaceConfig, logger *zap.Logger, metrics *metrics.Metrics) *NetworkStatsCollector {
	ctx, cancel := context.WithCancel(ctx)
	return &NetworkStatsCollector{
		config:  config,
		logger:  logger,
		metrics: metrics,
		ctx:     ctx,
		cancel:  cancel,
		stats:   make(map[string]*types.InterfaceStats),
	}
}

// Start begins collecting network statistics
func (c *NetworkStatsCollector) Start() error {
	if !c.config.StatCollection.Enabled {
		// c.logger.Info("Network statistics collection is disabled")
		return nil
	}

	// c.logger.Info("Starting network statistics",
	// 	zap.Int("interval", c.config.StatCollection.Interval))

	ticker := time.NewTicker(time.Duration(c.config.StatCollection.Interval) * time.Second)
	defer ticker.Stop()

	// Collect initial stats
	if err := c.collectStats(); err != nil {
		c.logger.Error("Failed to collect initial network stats", zap.Error(err))
	}

	for {
		select {
		case <-ticker.C:
			if err := c.collectStats(); err != nil {
				c.logger.Error("Failed to collect network stats", zap.Error(err))
			}
		case <-c.ctx.Done():
			return nil
		}
	}
}

// Stop stops the statistics collection
func (c *NetworkStatsCollector) Stop() {
	c.logger.Info("Stopping network statistics")
	c.cancel()
}

// GetStats returns the current statistics for all interfaces
func (c *NetworkStatsCollector) GetStats() map[string]*types.InterfaceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Make a copy of the stats
	stats := make(map[string]*types.InterfaceStats)
	for iface, stat := range c.stats {
		statCopy := *stat
		stats[iface] = &statCopy
	}

	return stats
}

// collectStats collects statistics for all monitored interfaces
func (c *NetworkStatsCollector) collectStats() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}

	newStats := make(map[string]*types.InterfaceStats)

	for _, iface := range interfaces {
		// Skip interfaces based on configuration
		if !shouldMonitorInterface(iface.Name, iface.Flags, c.config) {
			continue
		}

		stats, err := utils.GetInterfaceStats(iface.Name)
		if err != nil {
			c.logger.Debug("Failed to get stats for interface",
				zap.String("interface", iface.Name),
				zap.Error(err))
			continue
		}

		// Calculate rates if we have previous stats
		c.mu.RLock()
		if prevStats, exists := c.stats[iface.Name]; exists {
			duration := stats.CollectedAt.Sub(prevStats.CollectedAt)
			if duration > 0 {
				stats.RxBytesRate = float64(stats.RxBytes-prevStats.RxBytes) / duration.Seconds()
				stats.TxBytesRate = float64(stats.TxBytes-prevStats.TxBytes) / duration.Seconds()
				stats.RxPacketsRate = float64(stats.RxPackets-prevStats.RxPackets) / duration.Seconds()
				stats.TxPacketsRate = float64(stats.TxPackets-prevStats.TxPackets) / duration.Seconds()
			}
		}
		c.mu.RUnlock()

		newStats[iface.Name] = stats
	}

	// Update stats
	c.mu.Lock()
	c.stats = newStats
	c.mu.Unlock()

	return nil
}

// shouldMonitorInterface checks if an interface should be monitored based on configuration
func shouldMonitorInterface(name string, flags net.Flags, config *config.InterfaceConfig) bool {
	// Check exclusion list
	for _, excluded := range config.ExcludeInterfaces {
		if name == excluded {
			return false
		}
	}

	// Get interface type
	ifaceType := utils.GetInterfaceType(name)

	// Check if interface type is included
	typeIncluded := false
	for _, includedType := range config.InterfaceTypes {
		if string(ifaceType) == includedType {
			typeIncluded = true
			break
		}
	}

	if !typeIncluded {
		return false
	}

	// Check if it's a physical interface (unless virtual interfaces are included)
	if !config.IncludeVirtual && !utils.IsPhysicalInterface(name, flags) {
		return false
	}

	return true
}
