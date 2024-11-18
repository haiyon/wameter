package monitor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"ip-monitor/types"
	"ip-monitor/utils"

	"go.uber.org/zap"
)

// initialCheck performs the first check and sends initial notification
func (m *Monitor) initialCheck() error {
	// Get current IPs
	currentState, err := m.getCurrentIPs()
	if err != nil {
		return fmt.Errorf("failed to get current IPs for initial notification: %w", err)
	}

	// Ensure the state is properly initialized
	if currentState.IPv4 == nil {
		currentState.IPv4 = make([]string, 0)
	}
	if currentState.IPv6 == nil {
		currentState.IPv6 = make([]string, 0)
	}
	currentState.UpdatedAt = time.Now()

	// Get external IP if enabled
	if m.config.CheckExternalIP {
		if externalIP, err := m.getExternalIP(m.ctx); err == nil {
			currentState.ExternalIP = externalIP
		} else {
			m.logger.Warn("Failed to get external IP for initial notification", zap.Error(err))
		}
	}

	// Save the current state as last state
	m.mu.Lock()
	m.lastState = *currentState
	m.mu.Unlock()

	// Update network stats
	m.metrics.UpdateNetworkStats(currentState)

	// Log the initial state
	m.logger.Info("Initial IP state",
		zap.Any("ipv4", currentState.IPv4),
		zap.Any("ipv6", currentState.IPv6),
		zap.String("external_ip", currentState.ExternalIP),
		zap.Time("updated_at", currentState.UpdatedAt))

	// Prepare initial notification message
	changes := []string{
		"Initial state notification",
	}
	if len(currentState.IPv4) > 0 {
		changes = append(changes, fmt.Sprintf("IPv4 Addresses: %v", currentState.IPv4))
	}
	if len(currentState.IPv6) > 0 {
		changes = append(changes, fmt.Sprintf("IPv6 Addresses: %v", currentState.IPv6))
	}
	if currentState.ExternalIP != "" {
		changes = append(changes, fmt.Sprintf("External IP: %s", currentState.ExternalIP))
	}

	// Create empty initial state for comparison
	emptyState := types.IPState{
		IPv4:       make([]string, 0),
		IPv6:       make([]string, 0),
		UpdatedAt:  time.Time{},
		ExternalIP: "",
	}

	// Send initial notification
	if err := m.notifier.NotifyIPChange(emptyState, *currentState, changes); err != nil {
		return fmt.Errorf("failed to send initial notification: %w", err)
	}

	// Save state after successful notification
	if err := m.saveState(); err != nil {
		m.logger.Error("Failed to save initial state",
			zap.Error(err),
			zap.String("interface", m.config.NetworkInterface))
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
			if ip4 := ipnet.IP.To4(); ip4 != nil && m.config.IPVersion.EnableIPv4 {
				state.IPv4 = append(state.IPv4, ip4.String())
			} else if ip6 := ipnet.IP.To16(); ip6 != nil && m.config.IPVersion.EnableIPv6 && utils.IsGlobalIPv6(ip6) {
				// Only add global IPv6 addresses
				state.IPv6 = append(state.IPv6, ip6.String())
			}
		}
	}

	// Log the found IPs
	m.logger.Debug("Found IP addresses",
		zap.Strings("ipv4", state.IPv4),
		zap.Strings("ipv6", state.IPv6),
		zap.String("interface", m.config.NetworkInterface))

	return state, nil
}

// getExternalIP gets the current external IP address
func (m *Monitor) getExternalIP(ctx context.Context) (string, error) {
	// Select providers based on configuration
	var providers []string
	ipVersion := "ipv4"

	if m.config.IPVersion.EnableIPv6 {
		if m.config.IPVersion.PreferIPv6 || !m.config.IPVersion.EnableIPv4 {
			providers = m.config.ExternalIPProviders.IPv6
			ipVersion = "ipv6"
			m.logger.Debug("Using IPv6 providers for external IP check",
				zap.Strings("providers", providers))
		}
	}

	if len(providers) == 0 && m.config.IPVersion.EnableIPv4 {
		providers = m.config.ExternalIPProviders.IPv4
		ipVersion = "ipv4"
		m.logger.Debug("Using IPv4 providers for external IP check",
			zap.Strings("providers", providers))
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
						zap.String("version", ipVersion),
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
					m.logger.Debug("Using most reported external IP",
						zap.String("ip", mostFrequentIP),
						zap.String("version", ipVersion),
						zap.Int("reports", maxCount))
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
	isIPv6Request := strings.Contains(provider, "6") || m.config.IPVersion.PreferIPv6
	if isIPv6Request {
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
