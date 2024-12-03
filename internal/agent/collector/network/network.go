package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/haiyon/wameter/internal/agent/config"
	"github.com/haiyon/wameter/internal/types"
	"github.com/haiyon/wameter/internal/utils"
	"go.uber.org/zap"
)

type Collector struct {
	config    *config.NetworkConfig
	logger    *zap.Logger
	stats     *statsCollector
	stopChan  chan struct{}
	lastState *types.NetworkState
	mu        sync.RWMutex
}

// NewCollector creates a new network collector
func NewCollector(cfg *config.NetworkConfig, logger *zap.Logger) *Collector {
	return &Collector{
		config:   cfg,
		logger:   logger,
		stopChan: make(chan struct{}),
		stats:    newStatsCollector(cfg, logger),
	}
}

// Name returns the collector name
func (c *Collector) Name() string {
	return "network"
}

// Start starts the collector
func (c *Collector) Start(ctx context.Context) error {
	if !c.config.Enabled {
		c.logger.Info("Network collector is disabled")
		return nil
	}

	// Start statistics collector
	if err := c.stats.Start(ctx); err != nil {
		return fmt.Errorf("failed to start stats collector: %w", err)
	}

	return nil
}

// Stop stops the collector
func (c *Collector) Stop() error {
	close(c.stopChan)
	return c.stats.Stop()
}

// Collect performs a single collection
func (c *Collector) Collect(ctx context.Context) (*types.MetricsData, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	state := &types.NetworkState{
		Hostname:    hostname,
		Timestamp:   time.Now(),
		Interfaces:  make(map[string]*types.InterfaceInfo),
		CollectedAt: time.Now(),
	}

	// Collect interface information
	if err := c.collectInterfaces(state); err != nil {
		return nil, fmt.Errorf("failed to collect interface info: %w", err)
	}

	// Collect external IP if enabled
	if c.config.CheckExternalIP {
		if ip, err := c.getExternalIP(ctx); err == nil {
			state.ExternalIP = ip
		} else {
			c.logger.Warn("Failed to get external IP", zap.Error(err))
		}
	}

	// Get interface statistics
	stats := c.stats.GetStats()
	for name, ifaceInfo := range state.Interfaces {
		if stat, ok := stats[name]; ok {
			ifaceInfo.Statistics = stat
		}
	}

	c.mu.Lock()
	c.lastState = state
	c.mu.Unlock()

	return &types.MetricsData{
		Timestamp:   state.Timestamp,
		Hostname:    hostname,
		CollectedAt: state.CollectedAt,
		Metrics: struct {
			Network *types.NetworkState `json:"network,omitempty"`
		}{
			Network: state,
		},
	}, nil
}

func (c *Collector) collectInterfaces(state *types.NetworkState) error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	for _, iface := range interfaces {
		// Skip interfaces based on configuration
		if !c.shouldMonitorInterface(iface) {
			continue
		}

		info := &types.InterfaceInfo{
			Name:      iface.Name,
			Type:      string(utils.GetInterfaceType(iface.Name)),
			MAC:       iface.HardwareAddr.String(),
			MTU:       iface.MTU,
			Flags:     iface.Flags.String(),
			IPv4:      make([]string, 0),
			IPv6:      make([]string, 0),
			UpdatedAt: time.Now(),
		}

		addrs, err := iface.Addrs()
		if err != nil {
			c.logger.Warn("Failed to get addresses",
				zap.String("interface", iface.Name),
				zap.Error(err))
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					info.IPv4 = append(info.IPv4,
						fmt.Sprintf("%s/%d", ip4.String(),
							utils.NetworkMaskSize(ipnet.Mask)))
				} else if ip6 := ipnet.IP.To16(); ip6 != nil {
					info.IPv6 = append(info.IPv6,
						fmt.Sprintf("%s/%d", ip6.String(),
							utils.NetworkMaskSize(ipnet.Mask)))
				}
			}
		}

		state.Interfaces[iface.Name] = info
	}

	return nil
}

func (c *Collector) shouldMonitorInterface(iface net.Interface) bool {
	// Check if interface is in configured list
	if len(c.config.Interfaces) > 0 {
		found := false
		for _, name := range c.config.Interfaces {
			if name == iface.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check exclusion patterns
	for _, pattern := range c.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, iface.Name); matched {
			return false
		}
	}

	// Skip virtual interfaces unless explicitly enabled
	if !c.config.IncludeVirtual && utils.IsVirtualInterface(iface.Name) {
		return false
	}

	return true
}

// getExternalIP attempts to get the external IP using configured providers
func (c *Collector) getExternalIP(ctx context.Context) (string, error) {
	if len(c.config.ExternalProviders) == 0 {
		return "", fmt.Errorf("no external IP providers configured")
	}

	type result struct {
		provider string
		ip       string
		err      error
	}

	// Create channel for results
	results := make(chan result, len(c.config.ExternalProviders))

	// Query all providers concurrently
	for _, provider := range c.config.ExternalProviders {
		go func(p string) {
			ip, err := c.queryExternalProvider(ctx, p)
			results <- result{p, ip, err}
		}(provider)
	}

	// Collect results with timeout
	timeout := time.After(10 * time.Second)
	var lastErr error
	ips := make(map[string]int)

	for i := 0; i < len(c.config.ExternalProviders); i++ {
		select {
		case r := <-results:
			if r.err != nil {
				lastErr = r.err
				continue
			}
			ips[r.ip]++
			// If we have a consensus among multiple providers, return that IP
			if count := ips[r.ip]; count >= 2 {
				return r.ip, nil
			}
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for external IP providers")
		}
	}

	// If we have at least one successful result
	if len(ips) > 0 {
		// Return the most reported IP
		var mostReportedIP string
		maxCount := 0
		for ip, count := range ips {
			if count > maxCount {
				mostReportedIP = ip
				maxCount = count
			}
		}
		return mostReportedIP, nil
	}

	return "", fmt.Errorf("failed to get external IP: %v", lastErr)
}

// queryExternalProvider queries a single external IP provider
func (c *Collector) queryExternalProvider(ctx context.Context, provider string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "wameter-agent/1.0")
	req.Header.Set("Accept", "text/plain")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	ip := strings.TrimSpace(string(body))
	if !utils.IsValidIP(ip) {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	return ip, nil
}
