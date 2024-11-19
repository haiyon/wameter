package config

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
)

const appName = "ip-monitor"

// Config represents the application configuration
type Config struct {
	CheckInterval       int               `json:"check_interval"`        // Interval between checks in seconds
	IPVersion           IPVersionConfig   `json:"ip_version"`            // IP version configuration
	EmailConfig         *Email            `json:"email"`                 // Email notification settings
	TelegramConfig      *Telegram         `json:"telegram,omitempty"`    // Telegram notification settings
	LogConfig           LogConfig         `json:"log"`                   // Logging configuration
	LastIPFile          string            `json:"last_ip_file"`          // File to store the last known IP
	Debug               bool              `json:"debug"`                 // Enable debug logging
	CheckExternalIP     bool              `json:"check_external_ip"`     // Enable external IP checking
	ExternalIPProviders ExternalProviders `json:"external_ip_providers"` // List of external IP providers
	InterfaceConfig     *InterfaceConfig  `json:"interface_config"`      // Interface configuration
}

// IPVersionConfig represents IP version configuration
type IPVersionConfig struct {
	EnableIPv4 bool `json:"enable_ipv4"` // Enable IPv4
	EnableIPv6 bool `json:"enable_ipv6"` // Enable IPv6
	PreferIPv6 bool `json:"prefer_ipv6"` // Prefer IPv6
}

// ExternalProviders represents external IP providers
type ExternalProviders struct {
	IPv4 []string `json:"ipv4"` // List of IPv4 providers
	IPv6 []string `json:"ipv6"` // List of IPv6 providers
}

// InterfaceConfig represents interface monitoring configuration
type InterfaceConfig struct {
	IncludeVirtual    bool                  `json:"include_virtual"`
	ExcludeInterfaces []string              `json:"exclude_interfaces"`
	InterfaceTypes    []string              `json:"interface_types"`
	StatCollection    *StatCollectionConfig `json:"stat_collection"`
}

// StatCollectionConfig represents interface statistics collection configuration
type StatCollectionConfig struct {
	Enabled      bool     `json:"enabled"`
	Interval     int      `json:"interval"`
	IncludeStats []string `json:"include_stats"`
}

// Email configuration
type Email struct {
	Enabled    bool     `json:"enabled"`     // Enable email notifications
	SMTPServer string   `json:"smtp_server"` // SMTP server
	SMTPPort   int      `json:"smtp_port"`   // SMTP port
	Username   string   `json:"username"`    // SMTP username
	Password   string   `json:"password"`    // SMTP password
	From       string   `json:"from"`        // Sender
	To         []string `json:"to"`          // Recipients
	UseTLS     bool     `json:"use_tls"`     // Use TLS for SMTP connection
}

// Telegram configuration
type Telegram struct {
	Enabled  bool     `json:"enabled"`   // Enable Telegram notifications
	BotToken string   `json:"bot_token"` // Telegram bot token
	ChatIDs  []string `json:"chat_ids"`  // List of chat IDs to send notifications to
}

// LogConfig represents logging configuration
type LogConfig struct {
	Directory       string `json:"directory"`         // Log directory
	Filename        string `json:"filename"`          // Log filename
	MaxSize         int    `json:"max_size"`          // Maximum size of log file in MB
	MaxBackups      int    `json:"max_backups"`       // Maximum number of old log files
	MaxAge          int    `json:"max_age"`           // Maximum days to retain old log files
	Compress        bool   `json:"compress"`          // Compress old log files
	LogLevel        string `json:"level"`             // Log level (debug, info, warn, error)
	RotateOnStartup bool   `json:"rotate_on_startup"` // Rotate log files on startup
	TimeFormat      string `json:"time_format"`       // Time format
	UseLocalTime    bool   `json:"use_local_time"`    // Use local time
}

// LoadConfig loads configuration from file
func LoadConfig(customPath string) (*Config, error) {
	// First find the config file
	configPath, err := findConfigFile(customPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find config file: %w", err)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Set default values
	if err := config.setDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() error {
	if c.CheckInterval == 0 {
		c.CheckInterval = 300 // 5 minutes
	}

	if len(c.ExternalIPProviders.IPv4) == 0 {
		c.ExternalIPProviders.IPv4 = []string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
		}
	}

	if len(c.ExternalIPProviders.IPv6) == 0 {
		c.ExternalIPProviders.IPv6 = []string{
			"https://api6.ipify.org",
			"https://v6.ident.me",
		}
	}

	// Set default interface configuration
	if c.InterfaceConfig == nil {
		c.InterfaceConfig = &InterfaceConfig{
			IncludeVirtual: false,
			ExcludeInterfaces: []string{
				"lo",
				"docker0",
			},
			InterfaceTypes: []string{
				"ethernet",
				"wireless",
			},
			StatCollection: &StatCollectionConfig{
				Enabled:  true,
				Interval: 60,
				IncludeStats: []string{
					"rx_bytes",
					"tx_bytes",
					"rx_packets",
					"tx_packets",
					"rx_errors",
					"tx_errors",
					"rx_dropped",
					"tx_dropped",
				},
			},
		}
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.CheckInterval < 60 {
		return fmt.Errorf("check_interval must be at least 60 seconds")
	}

	// Enable at least one IP version
	if !c.IPVersion.EnableIPv4 && !c.IPVersion.EnableIPv6 {
		return fmt.Errorf("at least one IP version must be enabled")
	}

	// Validate interface configuration
	if c.InterfaceConfig == nil {
		return fmt.Errorf("interface configuration cannot be empty")
	}

	// Validate interface types
	if len(c.InterfaceConfig.InterfaceTypes) == 0 {
		return fmt.Errorf("at least one interface type must be configured")
	}

	// Validate that configured interface types are valid
	for _, ifaceType := range c.InterfaceConfig.InterfaceTypes {
		if !isValidInterfaceType(ifaceType) {
			return fmt.Errorf("invalid interface type: %s", ifaceType)
		}
	}

	// Validate external IP providers
	if c.CheckExternalIP {
		if c.IPVersion.EnableIPv4 {
			if err := validateProviders(c.ExternalIPProviders.IPv4); err != nil {
				return fmt.Errorf("invalid IPv4 providers: %w", err)
			}
		}

		if c.IPVersion.EnableIPv6 {
			if err := validateProviders(c.ExternalIPProviders.IPv6); err != nil {
				return fmt.Errorf("invalid IPv6 providers: %w", err)
			}
		}

		// Ensure at least one provider is configured for enabled IP versions
		if c.IPVersion.EnableIPv4 && len(c.ExternalIPProviders.IPv4) == 0 {
			return fmt.Errorf("IPv4 is enabled but no IPv4 providers configured")
		}
		if c.IPVersion.EnableIPv6 && len(c.ExternalIPProviders.IPv6) == 0 {
			return fmt.Errorf("IPv6 is enabled but no IPv6 providers configured")
		}
	}

	// Validate email configuration if enabled
	if c.EmailConfig != nil && c.EmailConfig.Enabled {
		if err := validateEmailConfig(c.EmailConfig); err != nil {
			return fmt.Errorf("invalid email configuration: %w", err)
		}
	}

	// Validate telegram configuration if enabled
	if c.TelegramConfig != nil && c.TelegramConfig.Enabled {
		if err := validateTelegramConfig(c.TelegramConfig); err != nil {
			return fmt.Errorf("invalid telegram configuration: %w", err)
		}
	}

	// Set default last IP file
	c.LastIPFile = filepath.Join(c.LogConfig.Directory, "last_ip.json")

	// Validate log configuration
	if err := validateLogConfig(&c.LogConfig); err != nil {
		return fmt.Errorf("invalid log configuration: %w", err)
	}

	return nil
}

// isValidInterfaceType checks if the interface type is valid
func isValidInterfaceType(ifaceType string) bool {
	validTypes := map[string]bool{
		"ethernet": true,
		"wireless": true,
		"bridge":   true,
		"virtual":  true,
		"tunnel":   true,
		"bonding":  true,
		"vlan":     true,
	}
	return validTypes[ifaceType]
}

func validateProviders(providers []string) error {
	if len(providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	for _, provider := range providers {
		// Check if URL is empty
		if provider == "" {
			return fmt.Errorf("provider URL cannot be empty")
		}

		// Parse and validate URL
		u, err := url.ParseRequestURI(provider)
		if err != nil {
			return fmt.Errorf("invalid provider URL %s: %w", provider, err)
		}

		// Validate scheme
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("provider URL %s must use HTTP(S) protocol", provider)
		}

		// Validate host
		if u.Host == "" {
			return fmt.Errorf("provider URL %s has no host", provider)
		}

		// Check for fragments (which shouldn't exist in API URLs)
		if u.Fragment != "" {
			return fmt.Errorf("provider URL %s should not contain fragments", provider)
		}

		// Check for query parameters (optional, depending on your API requirements)
		if u.RawQuery != "" {
			return fmt.Errorf("provider URL %s should not contain query parameters", provider)
		}
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, provider := range providers {
		if seen[provider] {
			return fmt.Errorf("duplicate provider URL: %s", provider)
		}
		seen[provider] = true
	}

	return nil
}

// validateEmailConfig validates email configuration
func validateEmailConfig(config *Email) error {
	if config.SMTPServer == "" {
		return fmt.Errorf("SMTP server cannot be empty")
	}

	if config.SMTPPort <= 0 || config.SMTPPort > 65535 {
		return fmt.Errorf("invalid SMTP port: %d", config.SMTPPort)
	}

	if config.From == "" {
		return fmt.Errorf("sender email cannot be empty")
	}

	if _, err := mail.ParseAddress(config.From); err != nil {
		return fmt.Errorf("invalid sender email: %w", err)
	}

	if len(config.To) == 0 {
		return fmt.Errorf("recipient list cannot be empty")
	}

	for _, recipient := range config.To {
		if _, err := mail.ParseAddress(recipient); err != nil {
			return fmt.Errorf("invalid recipient email %s: %w", recipient, err)
		}
	}

	return nil
}

// validateTelegramConfig validates telegram configuration
func validateTelegramConfig(config *Telegram) error {
	if config.BotToken == "" {
		return fmt.Errorf("bot token cannot be empty")
	}

	if len(config.ChatIDs) == 0 {
		return fmt.Errorf("chat IDs list cannot be empty")
	}

	for _, chatID := range config.ChatIDs {
		if chatID == "" {
			return fmt.Errorf("chat ID cannot be empty")
		}
	}

	return nil
}

// validateLogConfig validates logging configuration
func validateLogConfig(config *LogConfig) error {
	if config.Directory == "" {
		return fmt.Errorf("log directory cannot be empty")
	}

	if config.Filename == "" {
		// Use default filename
		config.Filename = "ip_monitor.log"
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Validate log level
	switch config.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	return nil
}

// getConfigPaths get config paths
func getConfigPaths() []string {
	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	// Get config paths
	paths := []string{
		// 1. current directory
		"config.json",
		fmt.Sprintf("./%s.json", appName),

		// 2. User configuration directory (~/.config/ip-monitor/config.json)
		filepath.Join(homeDir, ".config", appName, "config.json"),

		// 3. System configuration directory
		fmt.Sprintf("/etc/%s/config.json", appName),
		fmt.Sprintf("/etc/%s.json", appName),

		// 4. Current directory
		filepath.Join(filepath.Dir(os.Args[0]), "config.json"),
	}

	return paths
}

// findConfigFile find first existing config file
func findConfigFile(customPath string) (string, error) {
	// If the custom path is specified
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
		return "", fmt.Errorf("specified config file not found: %s", customPath)
	}

	// Check standard locations
	for _, path := range getConfigPaths() {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no config file found in standard locations")
}
