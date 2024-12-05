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
	"wameter/internal/agent/reporter"
	"wameter/internal/notify"
	"wameter/internal/version"

	"wameter/internal/agent/config"
	"wameter/internal/types"
	"wameter/internal/utils"

	"go.uber.org/zap"
)

// networkCollector represents a network collector
type networkCollector struct {
	config     *config.NetworkConfig
	agentID    string
	logger     *zap.Logger
	stats      *statsCollector
	ipTracker  *IPTracker
	reporter   *reporter.Reporter
	notifier   *notify.Manager
	stopChan   chan struct{}
	lastState  *types.NetworkState
	mu         sync.RWMutex
	client     *http.Client
	standalone bool
}

// NewCollector creates a new network collector
func NewCollector(cfg *config.NetworkConfig, agentID string, reporter *reporter.Reporter, notifier *notify.Manager, standalone bool, logger *zap.Logger) *networkCollector {
	if cfg.IPTracker == nil {
		cfg.IPTracker = config.IPtrackerDefaultConfig()
	}
	return &networkCollector{
		config:     cfg,
		agentID:    agentID,
		logger:     logger,
		stopChan:   make(chan struct{}),
		ipTracker:  NewIPTracker(cfg.IPTracker, logger),
		reporter:   reporter,
		notifier:   notifier,
		standalone: standalone,
		stats:      newStatsCollector(cfg, logger),
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 10,
			},
		},
	}
}

// Name returns the collector name
func (c *networkCollector) Name() string {
	return "network"
}

// Start starts the collector
func (c *networkCollector) Start(ctx context.Context) error {
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
func (c *networkCollector) Stop() error {
	close(c.stopChan)

	if err := c.stats.Stop(); err != nil {
		c.logger.Error("Failed to stop stats collector", zap.Error(err))
	}

	// Cleanup HTTP client resources
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	return nil
}

// Collect performs a single collection
func (c *networkCollector) Collect(ctx context.Context) (*types.MetricsData, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	state := &types.NetworkState{
		AgentID:     c.agentID,
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
		AgentID:     c.agentID,
		Hostname:    hostname,
		Version:     version.GetInfo().Version,
		Timestamp:   state.Timestamp,
		CollectedAt: state.CollectedAt,
		ReportedAt:  time.Now(),
		Metrics: struct {
			Network *types.NetworkState `json:"network,omitempty"`
		}{
			Network: state,
		},
	}, nil
}

// collectInterfaces collects interface information
func (c *networkCollector) collectInterfaces(state *types.NetworkState) error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	ifaceStates := make(map[string]*types.IPState)

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

		ipState := &types.IPState{
			IPv4Addrs: make([]string, 0),
			IPv6Addrs: make([]string, 0),
			UpdatedAt: time.Now(),
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.IsUnspecified() || ipnet.IP.IsMulticast() {
					continue // Skip invalid IPs
				}
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					addr := fmt.Sprintf("%s/%d", ip4.String(),
						utils.NetworkMaskSize(ipnet.Mask))
					info.IPv4 = append(info.IPv4, addr)
					ipState.IPv4Addrs = append(ipState.IPv4Addrs, addr)
				} else if ip6 := ipnet.IP.To16(); ip6 != nil {
					addr := fmt.Sprintf("%s/%d", ip6.String(),
						utils.NetworkMaskSize(ipnet.Mask))
					info.IPv6 = append(info.IPv6, addr)
					ipState.IPv6Addrs = append(ipState.IPv6Addrs, addr)
				}
			}
		}

		state.Interfaces[iface.Name] = info
		ifaceStates[iface.Name] = ipState
	}

	// Track IP changes
	externalIPs := make(map[types.IPVersion]string)
	if state.ExternalIP != "" {
		if ip := net.ParseIP(state.ExternalIP); ip != nil {
			if ip.To4() != nil {
				externalIPs[types.IPv4] = state.ExternalIP
			} else {
				externalIPs[types.IPv6] = state.ExternalIP
			}
		}
	}

	changes := c.ipTracker.Track(ifaceStates, externalIPs)
	if len(changes) > 0 {
		c.handleIPChanges(changes)
	}

	return nil
}

// shouldMonitorInterface returns true if the interface should be monitored
func (c *networkCollector) shouldMonitorInterface(iface net.Interface) bool {
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
func (c *networkCollector) getExternalIP(ctx context.Context) (string, error) {
	if len(c.config.ExternalProviders) == 0 {
		return "", fmt.Errorf("no external IP providers configured")
	}

	type result struct {
		provider string
		ip       string
		err      error
	}

	// Create buffered channel to prevent goroutine leaks
	results := make(chan result, len(c.config.ExternalProviders))
	deadline := time.After(10 * time.Second)

	// Query all providers concurrently
	for _, provider := range c.config.ExternalProviders {
		go func(p string) {
			ip, err := c.queryExternalProvider(ctx, p)
			select {
			case results <- result{p, ip, err}:
			case <-ctx.Done():
			case <-deadline:
			}
		}(provider)
	}

	// Use map to track IP consensus
	ips := make(map[string]int)
	var lastErr error

	for i := 0; i < len(c.config.ExternalProviders); i++ {
		select {
		case r := <-results:
			if r.err != nil {
				lastErr = r.err
				continue
			}
			ips[r.ip]++
			if count := ips[r.ip]; count >= 2 {
				return r.ip, nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", fmt.Errorf("timeout waiting for external IP providers")
		}
	}

	// Return most reported IP if no consensus
	if len(ips) > 0 {
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
func (c *networkCollector) queryExternalProvider(ctx context.Context, provider string) (string, error) {
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

// handleIPChanges handles IP address changes
func (c *networkCollector) handleIPChanges(changes []types.IPChange) {
	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("Failed to get hostname", zap.Error(err))
		hostname = "unknown"
	}

	for _, change := range changes {
		// Log the change
		c.logger.Info("IP change detected",
			zap.String("agent_id", c.agentID),
			zap.String("hostname", hostname),
			zap.String("interface", change.InterfaceName),
			zap.String("version", string(change.Version)),
			zap.Bool("is_external", change.IsExternal))

		// Create agent info for notifications
		agent := &types.AgentInfo{
			ID:       c.agentID,
			Hostname: hostname,
			Status:   "online",
		}

		// In standalone mode or if local notifications are enabled, notify directly
		if c.standalone && c.notifier != nil && c.notifier.IsEnabled() {
			c.notifier.NotifyIPChange(agent, &change)
		}

		// If not in standalone mode, include changes in metrics for server notification
		if !c.standalone {
			data := &types.MetricsData{
				AgentID:     c.agentID,
				Hostname:    hostname,
				Timestamp:   change.Timestamp,
				CollectedAt: time.Now(),
				ReportedAt:  time.Now(),
				Metrics: struct {
					Network *types.NetworkState `json:"network,omitempty"`
				}{
					Network: &types.NetworkState{
						AgentID:    c.agentID,
						Hostname:   hostname,
						Timestamp:  change.Timestamp,
						IPChanges:  []types.IPChange{change},
						Interfaces: make(map[string]*types.InterfaceInfo),
					},
				},
			}

			// Report to server
			if err := c.reporter.Report(data); err != nil {
				c.logger.Error("Failed to report IP change",
					zap.Error(err))
			}
		}
	}
}
