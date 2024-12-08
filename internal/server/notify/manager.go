package notify

import (
	"context"
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
	// Check if notifications are enabled
	if !cfg.Enabled {
		return nil, nil
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

// NotifyAgentOffline sends agent offline notification
func (m *Manager) NotifyAgentOffline(agent *types.AgentInfo) {
	m.notifier.NotifyAgentOffline(agent)
}

// NotifyNetworkErrors sends network errors notification
func (m *Manager) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) {
	m.notifier.NotifyNetworkErrors(agentID, iface)
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (m *Manager) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) {
	m.notifier.NotifyHighNetworkUtilization(agentID, iface)
}

// NotifyIPChange sends IP change notification
func (m *Manager) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) {
	m.notifier.NotifyIPChange(agent, change)
}

// Check checks the health of the notification manager
func (m *Manager) Check(ctx context.Context) error {
	if m.notifier != nil {
		return m.notifier.Health(ctx)
	}
	return nil
}

// Close closes the notification manager
func (m *Manager) Close() error {
	if m.notifier != nil {
		return m.notifier.Stop()
	}
	return nil
}
