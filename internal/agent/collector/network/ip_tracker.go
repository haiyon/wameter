package network

import (
	"sort"
	"sync"
	"time"
	"wameter/internal/agent/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// IPTracker tracks IP address changes
type IPTracker struct {
	mu           sync.RWMutex
	lastState    map[string]*types.IPState  // interface -> IP state
	lastExternal map[types.IPVersion]string // version -> external IP
	lastSeen     map[string]time.Time       // interface -> last seen time
	config       *config.IPTrackerConfig
	logger       *zap.Logger
	metrics      *IPTrackerMetrics
}

// IPTrackerMetrics represents tracking metrics
type IPTrackerMetrics struct {
	TotalChanges     int64
	IPv4Changes      int64
	IPv6Changes      int64
	ExternalChanges  int64
	LastChangeTime   time.Time
	ChangesInWindow  int
	WindowStartTime  time.Time
	DroppedChanges   int64
	ExternalChecks   int64
	ExternalFailures int64
}

// NewIPTracker creates new IP tracker
func NewIPTracker(cfg *config.IPTrackerConfig, logger *zap.Logger) *IPTracker {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Set defaults if not specified
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 1 * time.Hour
	}
	if cfg.RetentionPeriod == 0 {
		cfg.RetentionPeriod = 24 * time.Hour
	}
	if cfg.ThresholdWindow == 0 {
		cfg.ThresholdWindow = 1 * time.Hour
	}
	if cfg.ChangeThreshold == 0 {
		cfg.ChangeThreshold = 10
	}
	if cfg.ExternalCheckTTL == 0 {
		cfg.ExternalCheckTTL = 5 * time.Minute
	}

	t := &IPTracker{
		lastState:    make(map[string]*types.IPState),
		lastExternal: make(map[types.IPVersion]string),
		lastSeen:     make(map[string]time.Time),
		config:       cfg,
		logger:       logger,
		metrics: &IPTrackerMetrics{
			WindowStartTime: time.Now(),
		},
	}

	// Start cleanup goroutine
	go t.cleanupLoop()

	return t
}

// Track checks for and returns IP changes
func (t *IPTracker) Track(interfaceState map[string]*types.IPState, externalIPs map[types.IPVersion]string) []types.IPChange {
	if interfaceState == nil {
		t.logger.Error("Received nil interface state")
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	var changes []types.IPChange
	now := time.Now()

	// Check rate limit
	if t.isRateLimited() {
		t.metrics.DroppedChanges++
		t.logger.Warn("Change tracking rate limited",
			zap.Int("changes_in_window", t.metrics.ChangesInWindow),
			zap.Duration("window_duration", now.Sub(t.metrics.WindowStartTime)))
		return nil
	}

	// Track external IP changes
	if len(externalIPs) > 0 {
		changes = append(changes, t.trackExternalChanges(externalIPs, now)...)
	}

	// Track interface IP changes
	for ifaceName, state := range interfaceState {
		// Update last seen time
		t.lastSeen[ifaceName] = now

		// Get or create last state
		lastState, exists := t.lastState[ifaceName]
		if !exists {
			if t.config.NotifyOnFirstSeen {
				if t.config.EnableIPv4 && len(state.IPv4Addrs) > 0 {
					changes = append(changes, types.IPChange{
						InterfaceName: ifaceName,
						Version:       types.IPv4,
						OldAddrs:      nil,
						NewAddrs:      state.IPv4Addrs,
						Timestamp:     now,
						Action:        types.IPChangeActionAdd,
						Reason:        "interface_added",
					})
				}
				if t.config.EnableIPv6 && len(state.IPv6Addrs) > 0 {
					changes = append(changes, types.IPChange{
						InterfaceName: ifaceName,
						Version:       types.IPv6,
						OldAddrs:      nil,
						NewAddrs:      state.IPv6Addrs,
						Timestamp:     now,
						Action:        types.IPChangeActionAdd,
						Reason:        "interface_added",
					})
				}
			}
			t.lastState[ifaceName] = state
			continue
		}

		// Check IPv4 changes if enabled
		if t.config.EnableIPv4 {
			if !equalIPs(lastState.IPv4Addrs, state.IPv4Addrs) {
				changes = append(changes, types.IPChange{
					InterfaceName: ifaceName,
					Version:       types.IPv4,
					OldAddrs:      lastState.IPv4Addrs,
					NewAddrs:      state.IPv4Addrs,
					Timestamp:     now,
					Action:        types.IPChangeActionUpdate,
					Reason:        "ipv4_changed",
				})
				t.metrics.IPv4Changes++
			}
		}

		// Check IPv6 changes if enabled
		if t.config.EnableIPv6 {
			if !equalIPs(lastState.IPv6Addrs, state.IPv6Addrs) {
				changes = append(changes, types.IPChange{
					InterfaceName: ifaceName,
					Version:       types.IPv6,
					OldAddrs:      lastState.IPv6Addrs,
					NewAddrs:      state.IPv6Addrs,
					Timestamp:     now,
					Action:        types.IPChangeActionUpdate,
					Reason:        "ipv6_changed",
				})
				t.metrics.IPv6Changes++
			}
		}
	}

	// Check for removed interfaces
	if t.config.NotifyOnRemoval {
		for name, oldState := range t.lastState {
			if _, exists := interfaceState[name]; !exists {
				// Interface was removed
				if t.config.EnableIPv4 && len(oldState.IPv4Addrs) > 0 {
					changes = append(changes, types.IPChange{
						InterfaceName: name,
						Version:       types.IPv4,
						OldAddrs:      oldState.IPv4Addrs,
						NewAddrs:      nil,
						Timestamp:     now,
						Action:        types.IPChangeActionRemove,
						Reason:        "interface_removed",
					})
				}
				if t.config.EnableIPv6 && len(oldState.IPv6Addrs) > 0 {
					changes = append(changes, types.IPChange{
						InterfaceName: name,
						Version:       types.IPv6,
						OldAddrs:      oldState.IPv6Addrs,
						NewAddrs:      nil,
						Timestamp:     now,
						Action:        types.IPChangeActionRemove,
						Reason:        "interface_removed",
					})
				}
				delete(t.lastState, name)
				delete(t.lastSeen, name)
			}
		}
	}

	// Update state after processing all changes
	for ifaceName, state := range interfaceState {
		t.lastState[ifaceName] = state
	}

	// Update metrics
	if len(changes) > 0 {
		t.updateMetrics(func(m *IPTrackerMetrics) {
			m.TotalChanges += int64(len(changes))
			m.LastChangeTime = now
			m.ChangesInWindow++
		})
	}

	return changes
}

// trackExternalChanges checks for external IP changes
func (t *IPTracker) trackExternalChanges(externalIPs map[types.IPVersion]string, now time.Time) []types.IPChange {
	var changes []types.IPChange

	for version, ip := range externalIPs {
		if lastIP, exists := t.lastExternal[version]; !exists {
			// First time seeing external IP
			if t.config.NotifyOnFirstSeen {
				changes = append(changes, types.IPChange{
					Version:    version,
					OldAddrs:   nil,
					NewAddrs:   []string{ip},
					IsExternal: true,
					Timestamp:  now,
					Action:     types.IPChangeActionAdd,
					Reason:     "external_ip_added",
				})
			}
		} else if lastIP != ip {
			// External IP changed
			changes = append(changes, types.IPChange{
				Version:    version,
				OldAddrs:   []string{lastIP},
				NewAddrs:   []string{ip},
				IsExternal: true,
				Timestamp:  now,
				Action:     types.IPChangeActionUpdate,
				Reason:     "external_ip_changed",
			})
			t.metrics.ExternalChanges++
		}
		t.lastExternal[version] = ip
	}

	// Check for removed external IPs
	if t.config.NotifyOnRemoval {
		for version, lastIP := range t.lastExternal {
			if _, exists := externalIPs[version]; !exists {
				changes = append(changes, types.IPChange{
					Version:    version,
					OldAddrs:   []string{lastIP},
					NewAddrs:   nil,
					IsExternal: true,
					Timestamp:  now,
					Action:     types.IPChangeActionRemove,
					Reason:     "external_ip_removed",
				})
				delete(t.lastExternal, version)
			}
		}
	}

	return changes
}

// isRateLimited checks if change tracking is currently rate limited
func (t *IPTracker) isRateLimited() bool {
	now := time.Now()

	// Reset window if needed
	if now.Sub(t.metrics.WindowStartTime) > t.config.ThresholdWindow {
		t.metrics.ChangesInWindow = 0
		t.metrics.WindowStartTime = now
	}

	return t.metrics.ChangesInWindow >= t.config.ChangeThreshold
}

// cleanupLoop periodically cleans up old state
func (t *IPTracker) cleanupLoop() {
	ticker := time.NewTicker(t.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		t.cleanup()
	}
}

// cleanup removes old interface state
func (t *IPTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	threshold := time.Now().Add(-t.config.RetentionPeriod)

	for ifaceName, lastSeen := range t.lastSeen {
		if lastSeen.Before(threshold) {
			delete(t.lastState, ifaceName)
			delete(t.lastSeen, ifaceName)
			t.logger.Debug("Cleaned up stale interface state",
				zap.String("interface", ifaceName),
				zap.Time("last_seen", lastSeen))
		}
	}
}

// equalIPs compares two slices of IP addresses
func equalIPs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Make copies for sorting
	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)

	sort.Strings(aCopy)
	sort.Strings(bCopy)

	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}

	return true
}

// Update metrics safely
func (t *IPTracker) updateMetrics(fn func(*IPTrackerMetrics)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(t.metrics)
}

// GetMetrics returns current metrics
func (t *IPTracker) GetMetrics() *IPTrackerMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return copy to avoid race conditions
	metrics := *t.metrics
	return &metrics
}

// Reset resets the tracker state
func (t *IPTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastState = make(map[string]*types.IPState)
	t.lastExternal = make(map[types.IPVersion]string)
	t.lastSeen = make(map[string]time.Time)
	t.metrics = &IPTrackerMetrics{
		WindowStartTime: time.Now(),
	}
}
