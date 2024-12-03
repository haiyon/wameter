package notify

import (
	"fmt"

	"wameter/internal/server/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Notifier represents notifier interface
type Notifier interface {
	NotifyAgentOffline(agent *types.AgentInfo) error
	NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error
	NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error
}

// Manager represents notifier manager
type Manager struct {
	config    *config.NotifyConfig
	logger    *zap.Logger
	notifiers []Notifier
}

// NewManager creates new notifier manager
func NewManager(cfg config.NotifyConfig, logger *zap.Logger) (*Manager, error) {
	m := &Manager{
		config: &cfg,
		logger: logger,
	}

	// Initialize email notifier if enabled
	if cfg.Email.Enabled {
		email, err := NewEmailNotifier(&cfg.Email, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize email notifier: %w", err)
		}
		m.notifiers = append(m.notifiers, email)
	}

	// Initialize telegram notifier if enabled
	if cfg.Telegram.Enabled {
		telegram, err := NewTelegramNotifier(&cfg.Telegram, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize telegram notifier: %w", err)
		}
		m.notifiers = append(m.notifiers, telegram)
	}

	return m, nil
}

// NotifyAgentOffline sends an agent offline notification
func (m *Manager) NotifyAgentOffline(agent *types.AgentInfo) {
	for _, n := range m.notifiers {
		if err := n.NotifyAgentOffline(agent); err != nil {
			m.logger.Error("Failed to send agent offline notification",
				zap.Error(err),
				zap.String("agent_id", agent.ID))
		}
	}
}

// NotifyNetworkErrors sends a network errors notification
func (m *Manager) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) {
	for _, n := range m.notifiers {
		if err := n.NotifyNetworkErrors(agentID, iface); err != nil {
			m.logger.Error("Failed to send network errors notification",
				zap.Error(err),
				zap.String("agent_id", agentID),
				zap.String("interface", iface.Name))
		}
	}
}

// NotifyHighNetworkUtilization sends a high network utilization notification
func (m *Manager) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) {
	for _, n := range m.notifiers {
		if err := n.NotifyHighNetworkUtilization(agentID, iface); err != nil {
			m.logger.Error("Failed to send high utilization notification",
				zap.Error(err),
				zap.String("agent_id", agentID),
				zap.String("interface", iface.Name))
		}
	}
}
