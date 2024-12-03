package config

import (
	"fmt"
	"time"

	"wameter/internal/server/storage"

	"github.com/spf13/viper"
)

// Config represents the complete server configuration
type Config struct {
	Server  ServerConfig   `mapstructure:"server"`
	Storage storage.Config `mapstructure:"storage"`
	Notify  NotifyConfig   `mapstructure:"notify"`
	API     APIConfig      `mapstructure:"api"`
	Log     LogConfig      `mapstructure:"log"`
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	Address      string        `mapstructure:"address"`
	MetricsPath  string        `mapstructure:"metrics_path"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	TLS          TLSConfig     `mapstructure:"tls"`
}

// TLSConfig represents the TLS configuration
type TLSConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	CertFile          string `mapstructure:"cert_file"`
	KeyFile           string `mapstructure:"key_file"`
	ClientCA          string `mapstructure:"client_ca"`
	MinVersion        string `mapstructure:"min_version"` // TLS1.2, TLS1.3
	MaxVersion        string `mapstructure:"max_version"`
	RequireClientCert bool   `mapstructure:"require_client_cert"`
}

// APIConfig represents the API configuration
type APIConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Address string `mapstructure:"address"`

	// Authentication
	Auth AuthConfig `mapstructure:"auth"`

	// CORS settings
	CORS CORSConfig `mapstructure:"cors"`

	// Rate limiting
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`

	// Metrics
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Documentation
	Docs DocsConfig `mapstructure:"docs"`
}

// AuthConfig represents the authentication configuration
type AuthConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Type         string        `mapstructure:"type"` // jwt, apikey, basic
	JWTSecret    string        `mapstructure:"jwt_secret"`
	JWTDuration  time.Duration `mapstructure:"jwt_duration"`
	AllowedUsers []string      `mapstructure:"allowed_users"`
}

// CORSConfig represents the CORS configuration
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	MaxAge           int      `mapstructure:"max_age"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

// RateLimitConfig represents the rate limiting configuration
type RateLimitConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Requests int           `mapstructure:"requests"`
	Window   time.Duration `mapstructure:"window"`
	Strategy string        `mapstructure:"strategy"` // token, leaky, sliding
}

// MetricsConfig represents the metrics configuration
type MetricsConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	Prometheus  bool   `mapstructure:"prometheus"`
	StatsdAddr  string `mapstructure:"statsd_addr"`
	ServiceName string `mapstructure:"service_name"`
}

// DocsConfig represents the documentation configuration
type DocsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Title   string `mapstructure:"title"`
	Version string `mapstructure:"version"`
}

// NotifyConfig represents the notification configuration
type NotifyConfig struct {
	Email    EmailConfig    `mapstructure:"email"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	Webhook  WebhookConfig  `mapstructure:"webhook"`
	Slack    SlackConfig    `mapstructure:"slack"`
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

// SlackConfig represents the slack notification configuration
type SlackConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
	Channel    string `mapstructure:"channel"`
	Username   string `mapstructure:"username"`
	IconEmoji  string `mapstructure:"icon_emoji"`
}

// LogConfig represents the logging configuration
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"` // json, console
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"` // MB
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"` // days
	Compress   bool   `mapstructure:"compress"`
	Color      bool   `mapstructure:"color"`
}

// LoadConfig loads server configuration from file
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Server.Address == "" {
		config.Server.Address = ":8080"
	}

	if config.Server.MetricsPath == "" {
		config.Server.MetricsPath = "/metrics"
	}

	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 * time.Second
	}

	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30 * time.Second
	}

	if config.API.RateLimit.Window == 0 {
		config.API.RateLimit.Window = time.Minute
	}

	if config.API.RateLimit.Requests == 0 {
		config.API.RateLimit.Requests = 60
	}

	if config.API.CORS.MaxAge == 0 {
		config.API.CORS.MaxAge = 86400
	}

	if config.Log.MaxSize == 0 {
		config.Log.MaxSize = 100
	}

	if config.Log.MaxBackups == 0 {
		config.Log.MaxBackups = 3
	}

	if config.Log.MaxAge == 0 {
		config.Log.MaxAge = 28
	}

	// Set default allowed methods for CORS
	if len(config.API.CORS.AllowedMethods) == 0 {
		config.API.CORS.AllowedMethods = []string{
			"GET", "POST", "PUT", "DELETE", "OPTIONS",
		}
	}

	// Set default allowed headers for CORS
	if len(config.API.CORS.AllowedHeaders) == 0 {
		config.API.CORS.AllowedHeaders = []string{
			"Content-Type", "Authorization", "X-Request-ID",
		}
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate storage configuration
	if err := validateStorageConfig(&config.Storage); err != nil {
		return fmt.Errorf("invalid storage config: %w", err)
	}

	// Validate TLS configuration
	if config.Server.TLS.Enabled {
		if err := validateTLSConfig(&config.Server.TLS); err != nil {
			return fmt.Errorf("invalid TLS config: %w", err)
		}
	}

	// Validate notification configuration
	if err := validateNotifyConfig(&config.Notify); err != nil {
		return fmt.Errorf("invalid notification config: %w", err)
	}

	// Validate API configuration
	if err := validateAPIConfig(&config.API); err != nil {
		return fmt.Errorf("invalid API config: %w", err)
	}

	return nil
}

// Validate storage configuration
func validateStorageConfig(config *storage.Config) error {
	switch config.Driver {
	case "sqlite", "mysql", "postgres":
		if config.DSN == "" {
			return fmt.Errorf("storage DSN is required")
		}
	default:
		return fmt.Errorf("unsupported storage driver: %s", config.Driver)
	}
	return nil
}

// Validate TLS configuration
func validateTLSConfig(config *TLSConfig) error {
	if config.CertFile == "" || config.KeyFile == "" {
		return fmt.Errorf("TLS cert and key files are required")
	}
	return nil
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

// Validate API configuration
func validateAPIConfig(config *APIConfig) error {
	if config.Auth.Enabled {
		if err := validateAuthConfig(&config.Auth); err != nil {
			return fmt.Errorf("invalid auth config: %w", err)
		}
	}
	return nil
}

// Validate auth configuration
func validateAuthConfig(config *AuthConfig) error {
	switch config.Type {
	case "jwt":
		if config.JWTSecret == "" {
			return fmt.Errorf("JWT secret is required")
		}
	case "apikey", "basic":
		if len(config.AllowedUsers) == 0 {
			return fmt.Errorf("allowed users list is required")
		}
	default:
		return fmt.Errorf("unsupported auth type: %s", config.Type)
	}
	return nil
}
