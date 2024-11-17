package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"ip-monitor/config"
	"ip-monitor/metrics"
	"ip-monitor/notifier"
	"ip-monitor/types"
	"ip-monitor/utils"

	"go.uber.org/zap"
)

// Monitor handles IP monitoring
type Monitor struct {
	config    *config.Config
	logger    *zap.Logger
	metrics   *metrics.Metrics
	notifier  *notifier.Notifier
	lastState types.IPState
	mu        sync.RWMutex
	client    *http.Client
	ctx       context.Context
	cancel    context.CancelFunc
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

	// Load last known state
	if err := m.loadState(); err != nil {
		logger.Warn("Failed to load last state", zap.Error(err))
	}

	return m, nil
}

// Start begins the monitoring process
func (m *Monitor) Start() error {
	m.logger.Info("Starting IP monitor",
		zap.String("interface", m.config.NetworkInterface),
		zap.Bool("external_ip_enabled", m.config.CheckExternalIP))

	// Create ticker for regular checks
	ticker := time.NewTicker(time.Duration(m.config.CheckInterval) * time.Second)
	defer ticker.Stop()

	// Perform initial check
	if err := m.checkIP(m.ctx); err != nil {
		m.logger.Error("Initial IP check failed", zap.Error(err))
	}

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
		m.logger.Debug("IP check completed",
			zap.Duration("duration", duration),
			zap.String("interface", m.config.NetworkInterface))
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
			m.logger.Warn("Failed to get external IP",
				zap.Error(err),
				zap.String("interface", m.config.NetworkInterface))
			// Don't return error here as internal IPs may have changed
		} else {
			currentState.ExternalIP = externalIP
			m.logger.Debug("Got external IP",
				zap.String("ip", externalIP),
				zap.String("interface", m.config.NetworkInterface))
		}
	}

	// Check for changes and handle them
	if changed, changes := m.hasIPChanged(currentState); changed {
		m.logger.Info("IP changes detected",
			zap.Strings("changes", changes),
			zap.String("interface", m.config.NetworkInterface))

		if err := m.handleIPChange(*currentState, changes); err != nil { // 修复：传递解引用的 currentState
			m.metrics.RecordError(err)
			return fmt.Errorf("failed to handle IP change: %w", err)
		}

		// Record metrics for IP changes
		m.metrics.RecordIPChange(&m.lastState, currentState) // currentState 已经是指针类型

		// Save state after successful change handling
		if err := m.saveState(); err != nil {
			m.logger.Error("Failed to save state",
				zap.Error(err),
				zap.String("interface", m.config.NetworkInterface))
		}
	}

	return nil
}

// getCurrentIPs retrieves all current IP addresses for the interface
func (m *Monitor) getCurrentIPs() (*types.IPState, error) {
	iface, err := net.InterfaceByName(m.config.NetworkInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w",
			m.config.NetworkInterface, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %w",
			m.config.NetworkInterface, err)
	}

	state := &types.IPState{
		UpdatedAt: time.Now(),
		IPv4:      make([]string, 0),
		IPv6:      make([]string, 0),
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				state.IPv4 = append(state.IPv4, ip4.String())
			} else if ip6 := ipnet.IP.To16(); ip6 != nil && utils.IsGlobalIPv6(ip6) {
				// Only add global IPv6 addresses
				state.IPv6 = append(state.IPv6, ip6.String())
			}
		}
	}

	return state, nil
}

// getExternalIP gets the current external IP address
func (m *Monitor) getExternalIP(ctx context.Context) (string, error) {
	// Select providers based on configuration
	var providers []string
	ipVersion := "ipv4"

	if m.config.IPVersion.PreferIPv6 && m.config.IPVersion.EnableIPv6 {
		providers = m.config.ExternalIPProviders.IPv6
		ipVersion = "ipv6"
	} else if m.config.IPVersion.EnableIPv4 {
		providers = m.config.ExternalIPProviders.IPv4
		ipVersion = "ipv4"
	}

	if len(providers) == 0 {
		return "", fmt.Errorf("no IP providers configured for %s", ipVersion)
	}

	// Create error channel for collecting results
	type result struct {
		provider string
		ip       string
		err      error
	}
	results := make(chan result, len(providers))

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Query all providers concurrently
	for _, provider := range providers {
		go func(p string) {
			ip, duration, err := m.fetchExternalIP(checkCtx, p)
			m.metrics.RecordProviderRequest(ipVersion, p, duration, err)
			results <- result{p, ip, err}
		}(provider)
	}

	// Collect results with timeout
	var lastErr error
	successCount := 0
	failureCount := 0
	receivedCount := 0
	ips := make(map[string]int) // Track IP frequency

	for {
		select {
		case <-checkCtx.Done():
			return "", fmt.Errorf("external IP check timed out: %w", checkCtx.Err())

		case r := <-results:
			receivedCount++

			if r.err != nil {
				failureCount++
				lastErr = r.err
				m.logger.Debug("Provider request failed",
					zap.String("provider", r.provider),
					zap.Error(r.err))
			} else {
				successCount++
				ips[r.ip]++

				// If we have a consensus among multiple providers, return that IP
				if count := ips[r.ip]; count >= 2 {
					m.logger.Debug("External IP consensus reached",
						zap.String("ip", r.ip),
						zap.Int("agreements", count))
					return r.ip, nil
				}
			}

			// Return first successful result if we've tried all providers
			if receivedCount == len(providers) {
				if successCount > 0 {
					// Find the most frequent IP
					var mostFrequentIP string
					maxCount := 0
					for ip, count := range ips {
						if count > maxCount {
							mostFrequentIP = ip
							maxCount = count
						}
					}
					return mostFrequentIP, nil
				}
				// All providers failed
				return "", fmt.Errorf("all providers failed, last error: %w", lastErr)
			}
		}
	}
}

// fetchExternalIP fetches external IP from a specific provider
func (m *Monitor) fetchExternalIP(ctx context.Context, provider string) (string, time.Duration, error) {
	start := time.Now()

	// Create request with timeout
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider, nil)
	if err != nil {
		return "", time.Since(start), fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("User-Agent", "ip-monitor/1.0")
	req.Header.Set("Accept", "text/plain")

	// Perform request
	resp, err := m.client.Do(req)
	if err != nil {
		return "", time.Since(start), fmt.Errorf("request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			m.logger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", time.Since(start), fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	// Read response with size limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", time.Since(start), fmt.Errorf("failed to read response: %w", err)
	}

	// Parse and validate IP
	ip := strings.TrimSpace(string(body))

	// Validate IP based on version
	if m.config.IPVersion.PreferIPv6 && m.config.IPVersion.EnableIPv6 {
		if !utils.IsValidIP(ip, true) {
			return "", time.Since(start), fmt.Errorf("invalid IPv6 address: %s", ip)
		}
	} else {
		if !utils.IsValidIP(ip) {
			return "", time.Since(start), fmt.Errorf("invalid IPv4 address: %s", ip)
		}
	}

	return ip, time.Since(start), nil
}

// hasIPChanged checks if IP addresses have changed
func (m *Monitor) hasIPChanged(current *types.IPState) (bool, []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var changes []string
	changed := false

	// Compare IPv4 addresses
	if !utils.StringSlicesEqual(m.lastState.IPv4, current.IPv4) {
		changes = append(changes, fmt.Sprintf("IPv4: %v -> %v",
			m.lastState.IPv4, current.IPv4))
		changed = true
	}

	// Compare IPv6 addresses
	if !utils.StringSlicesEqual(m.lastState.IPv6, current.IPv6) {
		changes = append(changes, fmt.Sprintf("IPv6: %v -> %v",
			m.lastState.IPv6, current.IPv6))
		changed = true
	}

	// Check external IP if enabled
	if m.config.CheckExternalIP &&
		current.ExternalIP != "" &&
		current.ExternalIP != m.lastState.ExternalIP {
		changes = append(changes, fmt.Sprintf("External IP: %s -> %s",
			m.lastState.ExternalIP, current.ExternalIP))
		changed = true
	}

	return changed, changes
}

// handleIPChange handles IP address changes
func (m *Monitor) handleIPChange(newState types.IPState, changes []string) error {

	// Log changes
	m.logger.Info("IP address changed",
		zap.Strings("changes", changes),
		zap.Time("time", newState.UpdatedAt))

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
