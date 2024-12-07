package config

import (
	"fmt"
	"strings"
	"time"
)

// NotifyConfig represents notification configuration
type NotifyConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Notification channels
	Email    EmailConfig    `mapstructure:"email"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	Webhook  WebhookConfig  `mapstructure:"webhook"`
	Slack    SlackConfig    `mapstructure:"slack"`
	WeChat   WeChatConfig   `mapstructure:"wechat"`
	DingTalk DingTalkConfig `mapstructure:"dingtalk"`
	Discord  DiscordConfig  `mapstructure:"discord"`
	Feishu   FeishuConfig   `mapstructure:"feishu"`

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

// FeishuConfig represents Feishu notification configuration
type FeishuConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	WebhookURL string            `mapstructure:"webhook_url"`
	Secret     string            `mapstructure:"secret"`
	Templates  map[string]string `mapstructure:"templates"`
}

// Validate notification configuration
func (cfg *NotifyConfig) Validate() error {
	if !cfg.Enabled {
		return nil
	}

	// Validate global settings
	if cfg.RetryAttempts < 0 {
		return fmt.Errorf("retry_attempts cannot be negative")
	}
	if cfg.RetryDelay <= 0 {
		return fmt.Errorf("retry_delay must be positive")
	}

	if cfg.Email.Enabled {
		if err := cfg.Email.Validate(); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
	}

	if cfg.Telegram.Enabled {
		if err := cfg.Telegram.Validate(); err != nil {
			return fmt.Errorf("invalid telegram config: %w", err)
		}
	}

	if cfg.Slack.Enabled {
		if err := cfg.Slack.Validate(); err != nil {
			return fmt.Errorf("invalid slack config: %w", err)
		}
	}

	if cfg.Discord.Enabled {
		if err := cfg.Discord.Validate(); err != nil {
			return fmt.Errorf("invalid discord config: %w", err)
		}
	}

	if cfg.DingTalk.Enabled {
		if err := cfg.DingTalk.Validate(); err != nil {
			return fmt.Errorf("invalid dingtalk config: %w", err)
		}
	}

	if cfg.WeChat.Enabled {
		if err := cfg.WeChat.Validate(); err != nil {
			return fmt.Errorf("invalid wechat config: %w", err)
		}
	}

	if cfg.Webhook.Enabled {
		if err := cfg.Webhook.Validate(); err != nil {
			return fmt.Errorf("invalid webhook config: %w", err)
		}
	}

	if cfg.Feishu.Enabled {
		if err := cfg.Feishu.Validate(); err != nil {
			return fmt.Errorf("invalid feishu config: %w", err)
		}
	}

	return nil
}

// Validate validates email configuration
func (cfg *EmailConfig) Validate() error {
	if cfg.SMTPServer == "" {
		return fmt.Errorf("SMTP server is required")
	}
	if cfg.From == "" {
		return fmt.Errorf("sender email is required")
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	if !strings.Contains(cfg.From, "@") {
		return fmt.Errorf("invalid sender email address: %s", cfg.From)
	}
	for _, to := range cfg.To {
		if !strings.Contains(to, "@") {
			return fmt.Errorf("invalid recipient email address: %s", to)
		}
	}
	return nil
}

// Validate validates telegram configuration
func (cfg *TelegramConfig) Validate() error {
	if cfg.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}
	if len(cfg.ChatIDs) == 0 {
		return fmt.Errorf("at least one chat ID is required")
	}
	return nil
}

// Validate validates slack configuration
func (cfg *SlackConfig) Validate() error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL is required")
	}
	return nil
}

// Validate validates discord configuration
func (cfg *DiscordConfig) Validate() error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("webhook_url is required")
	}
	return nil
}

// Validate validates dingtalk configuration
func (cfg *DingTalkConfig) Validate() error {
	if cfg.AccessToken == "" {
		return fmt.Errorf("access_token is required")
	}
	return nil
}

// Validate validates wechat configuration
func (cfg *WeChatConfig) Validate() error {
	if cfg.CorpID == "" {
		return fmt.Errorf("corp_id is required")
	}
	if cfg.AgentID == 0 {
		return fmt.Errorf("agent_id is required")
	}
	if cfg.Secret == "" {
		return fmt.Errorf("secret is required")
	}
	return nil
}

// Validate validates webhook configuration
func (cfg *WebhookConfig) Validate() error {
	if cfg.URL == "" {
		return fmt.Errorf("url is required")
	}
	if cfg.Method == "" {
		cfg.Method = "POST"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	return nil
}

// Validate validates Feishu configuration
func (cfg *FeishuConfig) Validate() error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.WebhookURL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	return nil
}
