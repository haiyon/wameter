package notify

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wameter/internal/config"
	"wameter/internal/notify/template"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// NotifierType represents the type of notifier
type NotifierType string

const (
	NotifierEmail    NotifierType = "email"
	NotifierTelegram NotifierType = "telegram"
	NotifierSlack    NotifierType = "slack"
	NotifierWeChat   NotifierType = "wechat"
	NotifierDingTalk NotifierType = "dingtalk"
	NotifierDiscord  NotifierType = "discord"
	NotifierWebhook  NotifierType = "webhook"
	NotifierFeishu   NotifierType = "feishu"
)

// Notifier represents notifier interface
type Notifier interface {
	// NotifyAgentOffline sends agent offline notification
	NotifyAgentOffline(agent *types.AgentInfo) error

	// NotifyNetworkErrors sends network errors notification
	NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error

	// NotifyHighNetworkUtilization sends high network utilization notification
	NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error

	// NotifyIPChange sends IP change notification
	NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error

	// Health checks the health of the notifier
	Health(ctx context.Context) error
}

// notification represents a notification to be sent
type notification struct {
	notifierType NotifierType
	notifyFunc   func(Notifier) error
}

// Manager represents notifier manager
type Manager struct {
	config      *config.NotifyConfig
	logger      *zap.Logger
	notifiers   map[NotifierType]Notifier
	mu          sync.RWMutex
	rateLimiter *RateLimiter
	tplLoader   *template.Loader
	notifyChan  chan notification
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// RateLimiter implements rate limiting for notifications
type RateLimiter struct {
	mu        sync.Mutex
	events    map[NotifierType][]time.Time
	interval  time.Duration
	maxEvents int
}

// AllowNotification checks if a notification is allowed under rate limits
func (r *RateLimiter) AllowNotification(notifierType NotifierType) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	timestamps := r.events[notifierType]

	// Clean expired timestamps
	valid := make([]time.Time, 0)
	for _, ts := range timestamps {
		if now.Sub(ts) < r.interval {
			valid = append(valid, ts)
		}
	}
	r.events[notifierType] = valid

	// Check if limit exceeded
	if len(valid) >= r.maxEvents {
		return false
	}

	// Add new timestamp
	r.events[notifierType] = append(r.events[notifierType], now)
	return true
}

// NewManager creates new notifier manager
func NewManager(cfg *config.NotifyConfig, logger *zap.Logger) (*Manager, error) {
	tplLoader, err := template.NewLoader(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template loader: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:    cfg,
		logger:    logger,
		notifiers: make(map[NotifierType]Notifier),
		tplLoader: tplLoader,
		rateLimiter: &RateLimiter{
			events:    make(map[NotifierType][]time.Time),
			interval:  cfg.RateLimit.Interval,
			maxEvents: cfg.RateLimit.MaxEvents,
		},
		notifyChan: make(chan notification, 100),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize enabled notifiers
	if cfg.Email.Enabled {
		if n, err := NewEmailNotifier(&cfg.Email, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierEmail] = n
		} else {
			logger.Error("Failed to initialize email notifier", zap.Error(err))
		}
	}

	if cfg.Telegram.Enabled {
		if n, err := NewTelegramNotifier(&cfg.Telegram, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierTelegram] = n
		} else {
			logger.Error("Failed to initialize telegram notifier", zap.Error(err))
		}
	}

	if cfg.Slack.Enabled {
		if n, err := NewSlackNotifier(&cfg.Slack, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierSlack] = n
		} else {
			logger.Error("Failed to initialize slack notifier", zap.Error(err))
		}
	}

	if cfg.WeChat.Enabled {
		if n, err := NewWeChatNotifier(&cfg.WeChat, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierWeChat] = n
		} else {
			logger.Error("Failed to initialize wechat notifier", zap.Error(err))
		}
	}

	if cfg.DingTalk.Enabled {
		if n, err := NewDingTalkNotifier(&cfg.DingTalk, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierDingTalk] = n
		} else {
			logger.Error("Failed to initialize dingtalk notifier", zap.Error(err))
		}
	}

	if cfg.Discord.Enabled {
		if n, err := NewDiscordNotifier(&cfg.Discord, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierDiscord] = n
		} else {
			logger.Error("Failed to initialize discord notifier", zap.Error(err))
		}
	}

	if cfg.Webhook.Enabled {
		if n, err := NewWebhookNotifier(&cfg.Webhook, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierWebhook] = n
		} else {
			logger.Error("Failed to initialize webhook notifier", zap.Error(err))
		}
	}

	if cfg.Feishu.Enabled {
		if n, err := NewFeishuNotifier(&cfg.Feishu, m.tplLoader, logger); err == nil {
			m.notifiers[NotifierFeishu] = n
		} else {
			logger.Error("Failed to initialize feishu notifier", zap.Error(err))
		}
	}

	// Start notification processor
	m.wg.Add(1)
	go m.processNotifications()

	return m, nil
}

// processNotifications handles notification sending in background
func (m *Manager) processNotifications() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case n := <-m.notifyChan:
			m.mu.RLock()
			notifier, ok := m.notifiers[n.notifierType]
			m.mu.RUnlock()

			if !ok {
				continue
			}

			if !m.rateLimiter.AllowNotification(n.notifierType) {
				m.logger.Warn("Rate limit exceeded for notifier",
					zap.String("type", string(n.notifierType)))
				continue
			}

			if err := n.notifyFunc(notifier); err != nil {
				m.logger.Error("Failed to send notification",
					zap.String("type", string(n.notifierType)),
					zap.Error(err))
			}
		}
	}
}

// NotifyAgentOffline sends an agent offline notification
func (m *Manager) NotifyAgentOffline(agent *types.AgentInfo) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for t := range m.notifiers {
		notifyType := t // Capture for closure
		m.notifyChan <- notification{
			notifierType: notifyType,
			notifyFunc: func(n Notifier) error {
				return n.NotifyAgentOffline(agent)
			},
		}
	}
}

// NotifyNetworkErrors sends a network errors notification
func (m *Manager) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for t := range m.notifiers {
		notifyType := t // Capture for closure
		m.notifyChan <- notification{
			notifierType: notifyType,
			notifyFunc: func(n Notifier) error {
				return n.NotifyNetworkErrors(agentID, iface)
			},
		}
	}
}

// NotifyHighNetworkUtilization sends a high network utilization notification
func (m *Manager) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for t := range m.notifiers {
		notifyType := t // Capture for closure
		m.notifyChan <- notification{
			notifierType: notifyType,
			notifyFunc: func(n Notifier) error {
				return n.NotifyHighNetworkUtilization(agentID, iface)
			},
		}
	}
}

// NotifyIPChange sends an IP change notification
func (m *Manager) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for t := range m.notifiers {
		notifyType := t
		m.notifyChan <- notification{
			notifierType: notifyType,
			notifyFunc: func(n Notifier) error {
				return n.NotifyIPChange(agent, change)
			},
		}
	}
}

// Stop gracefully stops the notification manager
func (m *Manager) Stop() error {
	// Signal processNotifications to stop
	m.cancel()
	// Wait for all notifications to be processed
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for notifications to complete")
	}
}

// Health checks the health of the notification manager
func (m *Manager) Health(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for t := range m.notifiers {
		notifyType := t // Capture for closure
		m.notifyChan <- notification{
			notifierType: notifyType,
			notifyFunc: func(n Notifier) error {
				return n.Health(ctx)
			},
		}
	}

	return nil
}

// IsEnabled checks if a notifier is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled
}

// IsNotifierEnabled checks if a notifier is enabled
func (m *Manager) IsNotifierEnabled(notifierType NotifierType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.notifiers[notifierType]
	return ok
}
