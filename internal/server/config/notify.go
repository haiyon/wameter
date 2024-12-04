package config

import (
	"fmt"
	"strings"
	"time"
)

// NotifyConfig represents the notification configuration
type NotifyConfig struct {
	Email    EmailConfig    `mapstructure:"email"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	Webhook  WebhookConfig  `mapstructure:"webhook"`
	Slack    SlackConfig    `mapstructure:"slack"`
	WeChat   WeChatConfig   `mapstructure:"wechat"`
	DingTalk DingTalkConfig `mapstructure:"dingtalk"`
	Discord  DiscordConfig  `mapstructure:"discord"`

	// Global notification settings
	RetryAttempts int                   `mapstructure:"retry_attempts"`
	RetryDelay    time.Duration         `mapstructure:"retry_delay"`
	MaxBatchSize  int                   `mapstructure:"max_batch_size"`
	RateLimit     NotifyRateLimitConfig `mapstructure:"rate_limit"`
}

// NotifyRateLimitConfig represents rate limiting configuration
type NotifyRateLimitConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	Interval   time.Duration `mapstructure:"interval"`
	MaxEvents  int           `mapstructure:"max_events"`
	PerChannel bool          `mapstructure:"per_channel"`
}

// EmailConfig represents the email notification configuration
type EmailConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	SMTPServer string            `mapstructure:"smtp_server"`
	SMTPPort   int               `mapstructure:"smtp_port"`
	Username   string            `mapstructure:"username"`
	Password   string            `mapstructure:"password"`
	From       string            `mapstructure:"from"`
	To         []string          `mapstructure:"to"`
	UseTLS     bool              `mapstructure:"use_tls"`
	Templates  map[string]string `mapstructure:"templates"`
}

// TelegramConfig represents the telegram notification configuration
type TelegramConfig struct {
	Enabled  bool     `mapstructure:"enabled"`
	BotToken string   `mapstructure:"bot_token"`
	ChatIDs  []string `mapstructure:"chat_ids"`
	Format   string   `mapstructure:"format"` // text, html, markdown
}

// WebhookConfig represents the webhook notification configuration
type WebhookConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	URL        string            `mapstructure:"url"`
	Secret     string            `mapstructure:"secret"`
	Method     string            `mapstructure:"method"`
	Timeout    time.Duration     `mapstructure:"timeout"`
	MaxRetries int               `mapstructure:"max_retries"`
	Headers    map[string]string `mapstructure:"headers"`
	CommonData map[string]any    `mapstructure:"common_data"`
}

// SlackConfig represents Slack notification configuration
type SlackConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	WebhookURL string            `mapstructure:"webhook_url"`
	Channel    string            `mapstructure:"channel"`
	Username   string            `mapstructure:"username"`
	IconEmoji  string            `mapstructure:"icon_emoji"`
	IconURL    string            `mapstructure:"icon_url"`
	BotToken   string            `mapstructure:"bot_token"`
	Templates  map[string]string `mapstructure:"templates"`
}

// WeChatConfig represents WeChat Work notification configuration
type WeChatConfig struct {
	Enabled   bool              `mapstructure:"enabled"`
	CorpID    string            `mapstructure:"corp_id"`
	AgentID   int               `mapstructure:"agent_id"`
	Secret    string            `mapstructure:"secret"`
	ToUser    string            `mapstructure:"to_user"`
	ToParty   string            `mapstructure:"to_party"`
	ToTag     string            `mapstructure:"to_tag"`
	Templates map[string]string `mapstructure:"templates"`
}

// DingTalkConfig represents DingTalk notification configuration
type DingTalkConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	AccessToken string            `mapstructure:"access_token"`
	Secret      string            `mapstructure:"secret"`
	AtMobiles   []string          `mapstructure:"at_mobiles"`
	AtUserIds   []string          `mapstructure:"at_user_ids"`
	AtAll       bool              `mapstructure:"at_all"`
	Templates   map[string]string `mapstructure:"templates"`
}

// DiscordConfig represents Discord notification configuration
type DiscordConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	WebhookURL string            `mapstructure:"webhook_url"`
	Username   string            `mapstructure:"username"`
	AvatarURL  string            `mapstructure:"avatar_url"`
	Templates  map[string]string `mapstructure:"templates"`
}

// Validate notification configuration
func validateNotifyConfig(config *NotifyConfig) error {
	if config.Email.Enabled {
		if err := validateEmailConfig(&config.Email); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
	}
	if config.Telegram.Enabled {
		if err := validateTelegramConfig(&config.Telegram); err != nil {
			return fmt.Errorf("invalid telegram config: %w", err)
		}
	}
	if config.Slack.Enabled {
		if err := validateSlackConfig(&config.Slack); err != nil {
			return fmt.Errorf("invalid slack config: %w", err)
		}
	}
	if config.Webhook.Enabled {
		if err := validateWebhookConfig(&config.Webhook); err != nil {
			return fmt.Errorf("invalid webhook config: %w", err)
		}
	}
	return nil
}

// Validate email configuration
func validateEmailConfig(config *EmailConfig) error {
	if config.SMTPServer == "" {
		return fmt.Errorf("SMTP server is required")
	}
	if config.From == "" {
		return fmt.Errorf("sender email is required")
	}
	if len(config.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	if !strings.Contains(config.From, "@") {
		return fmt.Errorf("invalid sender email address: %s", config.From)
	}
	for _, to := range config.To {
		if !strings.Contains(to, "@") {
			return fmt.Errorf("invalid recipient email address: %s", to)
		}
	}
	return nil
}

// Validate telegram configuration
func validateTelegramConfig(config *TelegramConfig) error {
	if config.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}
	if len(config.ChatIDs) == 0 {
		return fmt.Errorf("at least one chat ID is required")
	}
	return nil
}

// Validate slack configuration
func validateSlackConfig(config *SlackConfig) error {
	if config.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL is required")
	}
	return nil
}

// Validate webhook configuration
func validateWebhookConfig(config *WebhookConfig) error {
	if config.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	return nil
}
