package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wameter/internal/config"
	"wameter/internal/retry"
	"wameter/internal/utils"

	"github.com/spf13/viper"
)

// Config represents agent configuration
type Config struct {
	Agent     AgentConfig          `mapstructure:"agent"`
	Collector CollectorConfig      `mapstructure:"collector"`
	Notify    *config.NotifyConfig `mapstructure:"notify"`
	Log       *config.LogConfig    `mapstructure:"log"`
	Retry     *retry.Config        `mapstructure:"retry"`
}

// AgentConfig represents agent configuration
type AgentConfig struct {
	ID         string       `mapstructure:"id"`
	Hostname   string       `mapstructure:"hostname"`
	Port       int          `mapstructure:"port"`
	Server     ServerConfig `mapstructure:"server"`
	Standalone bool         `mapstructure:"standalone"`
	Heartbeat  struct {
		Interval    time.Duration `mapstructure:"interval"`
		MaxFailures int           `mapstructure:"max_failures"`
	} `mapstructure:"heartbeat"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
	TLS     TLSConfig     `mapstructure:"tls"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

// CollectorConfig represents collector configuration
type CollectorConfig struct {
	Interval time.Duration     `mapstructure:"interval"`
	Network  NetworkConfig     `mapstructure:"network"`
	Metrics  MetricsConfig     `mapstructure:"metrics"`
	Filters  []FilterConfig    `mapstructure:"filters"`
	Tags     map[string]string `mapstructure:"tags"`
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	Enabled           bool             `mapstructure:"enabled"`
	Interfaces        []string         `mapstructure:"interfaces"`
	ExcludePatterns   []string         `mapstructure:"exclude_patterns"`
	IncludeVirtual    bool             `mapstructure:"include_virtual"`
	CheckExternalIP   bool             `mapstructure:"check_external_ip"`
	StatInterval      time.Duration    `mapstructure:"stat_interval"`
	ExternalProviders []string         `mapstructure:"external_providers"`
	IPTracker         *IPTrackerConfig `mapstructure:"ip_tracking"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

// FilterConfig represents filter configuration
type FilterConfig struct {
	Type    string            `mapstructure:"type"`
	Name    string            `mapstructure:"name"`
	Enabled bool              `mapstructure:"enabled"`
	Rules   map[string]string `mapstructure:"rules"`
}

// IPTrackerConfig represents IP tracking configuration
type IPTrackerConfig struct {
	EnableIPv4        bool          `json:"enable_ipv4"`
	EnableIPv6        bool          `json:"enable_ipv6"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`     // Cleanup interval
	RetentionPeriod   time.Duration `json:"retention_period"`     // Retention period
	ChangeThreshold   int           `json:"change_threshold"`     // Max changes in window
	ThresholdWindow   time.Duration `json:"threshold_window"`     // Time window for changes
	ExternalCheckTTL  time.Duration `json:"external_check_ttl"`   // External IP check frequency
	NotifyOnFirstSeen bool          `json:"notify_on_first_seen"` // Notify on first seen
	NotifyOnRemoval   bool          `json:"notify_on_removal"`    // Notify on removal
}

// IPtrackerDefaultConfig returns the default IP tracker configuration
func IPtrackerDefaultConfig() *IPTrackerConfig {
	return &IPTrackerConfig{
		EnableIPv4:        true,
		EnableIPv6:        true,
		CleanupInterval:   1 * time.Hour,
		RetentionPeriod:   24 * time.Hour,
		ChangeThreshold:   10,
		ThresholdWindow:   1 * time.Hour,
		ExternalCheckTTL:  5 * time.Minute,
		NotifyOnFirstSeen: true,
		NotifyOnRemoval:   true,
	}
}

// LoadConfig loads the agent configuration from file
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	// Add search paths
	v.AddConfigPath(config.InDot)
	v.AddConfigPath(config.InHome)
	v.AddConfigPath(config.InHomeDot)
	v.AddConfigPath(config.InEtc)
	// Add current working directory
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}
	v.AddConfigPath(filepath.Dir(ex))

	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults if not specified
	setDefaults(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values if not specified
func setDefaults(cfg *Config) {
	if cfg.Agent.Hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown-" + cfg.Agent.ID[:8]
		}
		cfg.Agent.Hostname = hostname
	}

	if cfg.Agent.ID == "" {
		// Generate a short hash of the hostname
		cfg.Agent.ID = utils.ShortHash(cfg.Agent.Hostname)
	}

	if cfg.Agent.Port == 0 {
		cfg.Agent.Port = 8081
	}

	if cfg.Collector.Interval == 0 {
		cfg.Collector.Interval = 60 * time.Second
	}

	if cfg.Agent.Port == 0 {
		cfg.Agent.Port = 8081
	}

	if cfg.Agent.Server.Timeout == 0 {
		cfg.Agent.Server.Timeout = 30 * time.Second
	}

	if len(cfg.Collector.Network.ExternalProviders) == 0 {
		cfg.Collector.Network.ExternalProviders = []string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
		}
	}

	// Set defaults for logger
	cfg.Log = cfg.Log.SetDefaults()

	// Set defaults for retry
	cfg.Retry = cfg.Retry.SetDefaults()
}

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}

	if !cfg.Agent.Standalone {
		if cfg.Agent.Server.Address == "" {
			return fmt.Errorf("server address is required when not in standalone mode")
		}
	}

	if cfg.Agent.Server.TLS.Enabled {
		if cfg.Agent.Server.TLS.CertFile == "" || cfg.Agent.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS cert and key files are required when TLS is enabled")
		}
	}

	if cfg.Collector.Network.Enabled {
		if len(cfg.Collector.Network.Interfaces) > 0 {
			hasValidInterface := false
			for _, iface := range cfg.Collector.Network.Interfaces {
				if iface != "" {
					hasValidInterface = true
					break
				}
			}
			if !hasValidInterface {
				return fmt.Errorf("if interfaces list is provided, at least one valid interface must be specified")
			}
		}
	}

	if cfg.Agent.Standalone && cfg.Notify.Enabled {
		if err := cfg.Notify.Validate(); err != nil {
			return fmt.Errorf("invalid notification config: %w", err)
		}
	}

	return nil
}
