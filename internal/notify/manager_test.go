package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"wameter/internal/config"
	"wameter/internal/types"
)

// TestNotificationManager tests the notification manager
func TestNotificationManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.NotifyConfig{
		Enabled: true,
		RateLimit: config.NotifyRateLimitConfig{
			Enabled:    true,
			Interval:   time.Minute,
			MaxEvents:  10,
			PerChannel: true,
		},
	}

	// Test cases for different notification types
	testCases := []struct {
		name        string
		setupConfig func(*config.NotifyConfig)
		testFunc    func(*testing.T, *Manager)
	}{
		{
			name: "Email Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.Email = config.EmailConfig{
					Enabled:    false,
					SMTPServer: "smtp.example.com",
					SMTPPort:   587,
					Username:   "test@example.com",
					Password:   "password",
					From:       "wameter@example.com",
					To:         []string{"admin@example.com"},
					UseTLS:     true,
				}
			},
			testFunc: testEmailNotification,
		},
		{
			name: "Telegram Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.Telegram = config.TelegramConfig{
					Enabled:  false,
					BotToken: "test-bot-token",
					ChatIDs:  []string{"123456789"},
					Format:   "markdown",
				}
			},
			testFunc: testTelegramNotification,
		},
		{
			name: "Slack Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.Slack = config.SlackConfig{
					Enabled:    false,
					WebhookURL: "https://hooks.slack.com/services/xxx/yyy/zzz",
					Channel:    "#monitoring",
					Username:   "WameterBot",
				}
			},
			testFunc: testSlackNotification,
		},
		{
			name: "Discord Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.Discord = config.DiscordConfig{
					Enabled:    false,
					WebhookURL: "https://discord.com/api/webhooks/xxx/yyy",
					Username:   "WameterBot",
				}
			},
			testFunc: testDiscordNotification,
		},
		{
			name: "DingTalk Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.DingTalk = config.DingTalkConfig{
					Enabled:     false,
					AccessToken: "test-access-token",
					Secret:      "test-secret",
				}
			},
			testFunc: testDingTalkNotification,
		},
		{
			name: "Feishu Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.Feishu = config.FeishuConfig{
					Enabled:    false,
					WebhookURL: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
					Secret:     "test-secret",
				}
			},
			testFunc: testFeishuNotification,
		},
		{
			name: "WeChat Work Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.WeChat = config.WeChatConfig{
					Enabled: false,
					CorpID:  "test-corp-id",
					AgentID: 123456,
					Secret:  "test-secret",
					ToUser:  "test-user",
					ToParty: "test-party",
					ToTag:   "test-tag",
				}
			},
			testFunc: testWeChatWorkNotification,
		},
		{
			name: "Webhook Notification",
			setupConfig: func(cfg *config.NotifyConfig) {
				cfg.Webhook = config.WebhookConfig{
					Enabled: false,
					URL:     "https://webhook.example.com/notify",
					Secret:  "test-secret",
					Headers: map[string]string{
						"X-Custom-Header": "test-value",
					},
				}
			},
			testFunc: testWebhookNotification,
		},
	}
	// Run tests for each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testCfg := *cfg
			tc.setupConfig(&testCfg)

			manager, err := NewManager(&testCfg, logger)
			require.NoError(t, err)
			require.NotNil(t, manager)

			tc.testFunc(t, manager)

			err = manager.Stop()
			assert.NoError(t, err)
		})
	}
}

// createTestAgent creates a test agent
func createTestAgent() *types.AgentInfo {
	return &types.AgentInfo{
		ID:           "test-agent",
		Hostname:     "test-host",
		Version:      "1.0.0",
		Status:       types.AgentStatusOnline,
		LastSeen:     time.Now(),
		RegisteredAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt:    time.Now(),
	}
}

// createTestInterface creates a test interface
func createTestInterface() *types.InterfaceInfo {
	return &types.InterfaceInfo{
		Name: "eth0",
		Type: "ethernet",
		Statistics: &types.InterfaceStats{
			RxBytes:     1000000,
			TxBytes:     500000,
			RxBytesRate: 1024,
			TxBytesRate: 512,
			RxErrors:    10,
			TxErrors:    5,
		},
	}
}

// createTestIPChange creates a test IP change
func createTestIPChange() *types.IPChange {
	return &types.IPChange{
		InterfaceName: "eth0",
		Version:       types.IPv4,
		OldAddrs:      []string{"192.168.1.100"},
		NewAddrs:      []string{"192.168.1.200"},
		IsExternal:    false,
		Timestamp:     time.Now(),
		Action:        types.IPChangeActionUpdate,
		Reason:        "ip_changed",
	}
}

// testNotification common send notification function
func testNotification(t *testing.T, manager *Manager) {
	agent := createTestAgent()
	iface := createTestInterface()
	change := createTestIPChange()

	// Test sending notifications
	manager.NotifyAgentOffline(agent)
	manager.NotifyNetworkErrors(agent.ID, iface)
	manager.NotifyHighNetworkUtilization(agent.ID, iface)
	manager.NotifyIPChange(agent, change)
}

// testEmailNotification sends email notification
func testEmailNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testTelegramNotification sends telegram notification
func testTelegramNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testSlackNotification sends slack notification
func testSlackNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testDiscordNotification sends discord notification
func testDiscordNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testDingTalkNotification sends dingtalk notification
func testDingTalkNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testFeishuNotification sends feishu notification
func testFeishuNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testWeChatWorkNotification sends wechat work notification
func testWeChatWorkNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}

// testWebhookNotification sends webhook notification
func testWebhookNotification(t *testing.T, manager *Manager) {
	testNotification(t, manager)
}
