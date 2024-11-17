package notifier

import (
	"errors"
	"fmt"

	"ip-monitor/config"
	"ip-monitor/types"

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
	var errs []error

	// Check if this is an initial notification
	isInitial := len(oldState.IPv4) == 0 && len(oldState.IPv6) == 0 && oldState.ExternalIP == ""

	// Send email notification
	if m.email != nil {
		if err := m.email.Send(oldState, newState, changes, m.config.NetworkInterface, isInitial); err != nil {
			errs = append(errs, fmt.Errorf("email notification failed: %w", err))
		}
	}

	// Send telegram notification
	if m.telegram != nil {
		if err := m.telegram.Send(oldState, newState, changes, isInitial); err != nil {
			errs = append(errs, fmt.Errorf("telegram notification failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
