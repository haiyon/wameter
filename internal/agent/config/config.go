package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	commonCfg "wameter/internal/config"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// Config represents agent configuration
type Config struct {
	Agent     AgentConfig     `mapstructure:"agent"`
	Collector CollectorConfig `mapstructure:"collector"`
	Log       LogConfig       `mapstructure:"log"`
}

// AgentConfig represents agent configuration
type AgentConfig struct {
	ID       string       `mapstructure:"id"`
	Hostname string       `mapstructure:"hostname"`
	Port     int          `mapstructure:"port"`
	Server   ServerConfig `mapstructure:"server"`
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
	Enabled           bool          `mapstructure:"enabled"`
	Interfaces        []string      `mapstructure:"interfaces"`
	ExcludePatterns   []string      `mapstructure:"exclude_patterns"`
	IncludeVirtual    bool          `mapstructure:"include_virtual"`
	CheckExternalIP   bool          `mapstructure:"check_external_ip"`
	StatInterval      time.Duration `mapstructure:"stat_interval"`
	ExternalProviders []string      `mapstructure:"external_providers"`
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

// LogConfig represents logging configuration
type LogConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// Custom duration type for YAML parsing
// type duration time.Duration

// // UnmarshalText implements encoding.TextUnmarshaler
// func (d *duration) UnmarshalText(text []byte) error {
// 	dur, err := time.ParseDuration(string(text))
// 	if err != nil {
// 		return err
// 	}
// 	*d = duration(dur)
// 	return nil
// }
//
// // Duration Convert duration to time.Duration
// func (d duration) Duration() time.Duration {
// 	return time.Duration(d)
// }

// LoadConfig loads the agent configuration from file
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	// Add search paths
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/" + commonCfg.AppName)
	v.AddConfigPath("$HOME/." + commonCfg.AppName)
	v.AddConfigPath("/etc/" + commonCfg.AppName)
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

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults if not specified
	setDefaults(&config)

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values if not specified
func setDefaults(config *Config) {
	if config.Agent.ID == "" {
		config.Agent.ID = uuid.New().String()
	}

	if config.Agent.Hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown-" + config.Agent.ID[:8]
		}
		config.Agent.Hostname = hostname
	}

	if config.Agent.Port == 0 {
		config.Agent.Port = 8081
	}

	if config.Collector.Interval == 0 {
		config.Collector.Interval = 60 * time.Second
	}

	if config.Agent.Port == 0 {
		config.Agent.Port = 8081
	}

	if config.Agent.Server.Timeout == 0 {
		config.Agent.Server.Timeout = 30 * time.Second
	}

	if len(config.Collector.Network.ExternalProviders) == 0 {
		config.Collector.Network.ExternalProviders = []string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
		}
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
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}

	if config.Agent.Server.Address == "" {
		return fmt.Errorf("server address is required")
	}

	if config.Agent.Server.Address == "" {
		return fmt.Errorf("server address is required")
	}

	if config.Agent.Server.TLS.Enabled {
		if config.Agent.Server.TLS.CertFile == "" || config.Agent.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS cert and key files are required when TLS is enabled")
		}
	}

	if config.Collector.Network.Enabled && len(config.Collector.Network.Interfaces) == 0 {
		return fmt.Errorf("at least one interface must be specified when network collector is enabled")
	}

	return nil
}
