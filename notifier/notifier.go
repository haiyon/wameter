package notifier

import (
	"errors"
	"fmt"

	"github.com/haiyon/wameter/config"
	"github.com/haiyon/wameter/types"
	"github.com/haiyon/wameter/utils"

	"go.uber.org/zap"
)

// Notifier handles notifications
type Notifier struct {
	config   *config.Config
	logger   *zap.Logger
	email    *EmailNotifier
	telegram *TelegramNotifier
}

// NewNotifier creates a new notification manager
func NewNotifier(cfg *config.Config, logger *zap.Logger) (*Notifier, error) {
	m := &Notifier{
		config: cfg,
		logger: logger,
	}

	// Initialize email notifier if enabled
	if cfg.EmailConfig != nil && cfg.EmailConfig.Enabled {
		email, err := NewEmailNotifier(cfg.EmailConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize email notifier: %w", err)
		}
		m.email = email
	}

	// Initialize telegram notifier if enabled
	if cfg.TelegramConfig != nil && cfg.TelegramConfig.Enabled {
		telegram, err := NewTelegramNotifier(cfg.TelegramConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize telegram notifier: %w", err)
		}
		m.telegram = telegram
	}

	return m, nil
}

// NotifyIPChange sends notifications about IP changes
func (m *Notifier) NotifyIPChange(oldState, newState types.IPState, changes []string) error {
	// Convert the changes into interface-specific changes
	interfaceChanges := m.processStateChanges(oldState, newState)

	if len(interfaceChanges) == 0 && oldState.ExternalIP == newState.ExternalIP {
		m.logger.Debug("No changes to notify")
		return nil
	}

	// Get notification options
	opts := getNotificationOptions(m.config, isInitialState(oldState))

	var errs []error

	// Send email notification
	if m.email != nil {
		if err := m.email.Send(oldState, newState, interfaceChanges, opts); err != nil {
			errs = append(errs, fmt.Errorf("email notification failed: %w", err))
		}
	}

	// Send telegram notification
	if m.telegram != nil {
		if err := m.telegram.Send(oldState, newState, interfaceChanges, opts); err != nil {
			errs = append(errs, fmt.Errorf("telegram notification failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// processStateChanges analyzes changes between old and new states
func (m *Notifier) processStateChanges(oldState, newState types.IPState) []InterfaceChange {
	var changes []InterfaceChange

	// Track processed interfaces to handle removals
	processedInterfaces := make(map[string]bool)

	// Process all interfaces in new state
	for ifaceName, newIfaceState := range newState.InterfaceInfo {
		processedInterfaces[ifaceName] = true

		change := InterfaceChange{
			Name:   ifaceName,
			Type:   string(utils.GetInterfaceType(ifaceName)),
			Status: getInterfaceStatus(newIfaceState),
			Stats:  newIfaceState.Statistics,
		}

		// Get changes for existing interfaces
		if oldIfaceState, exists := oldState.InterfaceInfo[ifaceName]; exists {
			ifaceChanges := detectInterfaceChanges(oldIfaceState, newIfaceState)
			if len(ifaceChanges) > 0 {
				change.Changes = ifaceChanges
				changes = append(changes, change)
			}
		} else {
			// New interface appeared
			change.Changes = []string{fmt.Sprintf("New interface detected: %s (%s)", ifaceName, change.Type)}
			changes = append(changes, change)
		}
	}

	// Check for removed interfaces
	for ifaceName, _ := range oldState.InterfaceInfo {
		if !processedInterfaces[ifaceName] {
			changes = append(changes, InterfaceChange{
				Name:    ifaceName,
				Type:    string(utils.GetInterfaceType(ifaceName)),
				Status:  "down",
				Changes: []string{fmt.Sprintf("Interface removed: %s", ifaceName)},
			})
		}
	}

	// Handle external IP changes
	if oldState.ExternalIP != newState.ExternalIP {
		changes = append(changes, InterfaceChange{
			Name: "External",
			Type: "external",
			Changes: []string{
				fmt.Sprintf("External IP changed: %s -> %s",
					oldState.ExternalIP, newState.ExternalIP),
			},
		})
	}

	return changes
}

// detectInterfaceChanges detects changes between old and new interface states
func detectInterfaceChanges(oldState, newState *types.InterfaceState) []string {
	var changes []string

	// Compare IPv4 addresses
	if !utils.StringSlicesEqual(oldState.IPv4, newState.IPv4) {
		changes = append(changes, fmt.Sprintf("IPv4: %v -> %v",
			oldState.IPv4, newState.IPv4))
	}

	// Compare IPv6 addresses
	if !utils.StringSlicesEqual(oldState.IPv6, newState.IPv6) {
		changes = append(changes, fmt.Sprintf("IPv6: %v -> %v",
			oldState.IPv6, newState.IPv6))
	}

	// Compare interface status
	oldStatus := getInterfaceStatus(oldState)
	newStatus := getInterfaceStatus(newState)
	if oldStatus != newStatus {
		changes = append(changes, fmt.Sprintf("Status changed: %s -> %s",
			oldStatus, newStatus))
	}

	// Compare statistics if available
	if oldState.Statistics != nil && newState.Statistics != nil {
		statChanges := detectStatisticsChanges(oldState.Statistics, newState.Statistics)
		changes = append(changes, statChanges...)
	}

	return changes
}

// detectStatisticsChanges detects significant changes in interface statistics
func detectStatisticsChanges(old, new *types.InterfaceStats) []string {
	var changes []string

	// Check for significant rate changes (more than 20% difference)
	if old.RxBytesRate > 0 && new.RxBytesRate > 0 {
		changePct := ((new.RxBytesRate - old.RxBytesRate) / old.RxBytesRate) * 100
		if abs(changePct) > 20 {
			changes = append(changes, fmt.Sprintf(
				"Receive rate changed by %.1f%% (%.2f MB/s -> %.2f MB/s)",
				changePct,
				old.RxBytesRate/1024/1024,
				new.RxBytesRate/1024/1024))
		}
	}

	if old.TxBytesRate > 0 && new.TxBytesRate > 0 {
		changePct := ((new.TxBytesRate - old.TxBytesRate) / old.TxBytesRate) * 100
		if abs(changePct) > 20 {
			changes = append(changes, fmt.Sprintf(
				"Transmit rate changed by %.1f%% (%.2f MB/s -> %.2f MB/s)",
				changePct,
				old.TxBytesRate/1024/1024,
				new.TxBytesRate/1024/1024))
		}
	}

	// Check for error increases
	if new.RxErrors > old.RxErrors || new.TxErrors > old.TxErrors {
		changes = append(changes, fmt.Sprintf(
			"Network errors increased (Rx: %d -> %d, Tx: %d -> %d)",
			old.RxErrors, new.RxErrors,
			old.TxErrors, new.TxErrors))
	}

	return changes
}

// getInterfaceStatus returns a string representation of interface status
func getInterfaceStatus(state *types.InterfaceState) string {
	if state.Statistics != nil && state.Statistics.IsUp {
		return "up"
	}
	if state.Statistics != nil && state.Statistics.OperState != "" {
		if state.Statistics.OperState == "up" {
			return "up"
		}
		return "down"
	}
	// Fallback status check based on flags
	if len(state.IPv4) > 0 || len(state.IPv6) > 0 {
		return "up"
	}
	return "down"
}

// isInitialState checks if this is the initial state (no previous addresses)
func isInitialState(state types.IPState) bool {
	for _, iface := range state.InterfaceInfo {
		if len(iface.IPv4) > 0 || len(iface.IPv6) > 0 {
			return false
		}
	}
	return true
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// notificationOptions holds options for sending notifications
type notificationOptions struct {
	showIPv4     bool
	showIPv6     bool
	showExternal bool
	isInitial    bool
}

// getNotificationOptions returns options for sending notifications
func getNotificationOptions(cfg *config.Config, isInitial bool) notificationOptions {
	return notificationOptions{
		showIPv4:     cfg.IPVersion.EnableIPv4,
		showIPv6:     cfg.IPVersion.EnableIPv6,
		showExternal: cfg.CheckExternalIP,
		isInitial:    isInitial,
	}
}
