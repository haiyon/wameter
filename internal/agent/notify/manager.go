package notify

import (
	"wameter/internal/config"
	"wameter/internal/notify"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Manager wraps the server notification manager for agent use
type Manager struct {
	notifier *notify.Manager
	logger   *zap.Logger
}

// NewManager creates a new notification manager for agent
func NewManager(cfg *config.NotifyConfig, logger *zap.Logger) (*Manager, error) {
	if !cfg.Enabled {
		return &Manager{logger: logger}, nil
	}

	notifier, err := notify.NewManager(cfg, logger)
	if err != nil {
		return nil, err
	}

	return &Manager{
		notifier: notifier,
		logger:   logger,
	}, nil
}

// Stop stops the notification manager
func (m *Manager) Stop() error {
	if m.notifier != nil {
		return m.notifier.Stop()
	}
	return nil
}

// IsEnabled checks if notifications are enabled
func (m *Manager) IsEnabled() bool {
	return m.notifier != nil && m.notifier.IsEnabled()
}

// NotifyIPChange sends IP change notification
func (m *Manager) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) {
	if !m.IsEnabled() {
		return
	}
	m.notifier.NotifyIPChange(agent, change)
}

// Close closes the notification manager
func (m *Manager) Close() error {
	if m.notifier != nil {
		return m.notifier.Stop()
	}
	return nil
}
