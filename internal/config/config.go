package config

import (
	"fmt"
	"time"
)

var (
	AppName = "wameter"
)

// LogConfig represents logging configuration
type LogConfig struct {
	Directory    string `yaml:"directory"`
	Filename     string `yaml:"filename"`
	MaxSize      int    `yaml:"max_size"` // MB
	MaxBackups   int    `yaml:"max_backups"`
	MaxAge       int    `yaml:"max_age"` // days
	Compress     bool   `yaml:"compress"`
	Level        string `yaml:"level"` // debug, info, warn, error
	TimeFormat   string `yaml:"time_format"`
	UseLocalTime bool   `yaml:"use_local_time"`
}

// IPVersionConfig represents IP version configuration
type IPVersionConfig struct {
	EnableIPv4 bool `yaml:"enable_ipv4"`
	EnableIPv6 bool `yaml:"enable_ipv6"`
	PreferIPv6 bool `yaml:"prefer_ipv6"`
}

// InterfaceConfig represents interface monitoring configuration
type InterfaceConfig struct {
	IncludeVirtual    bool                  `yaml:"include_virtual"`
	ExcludeInterfaces []string              `yaml:"exclude_interfaces"`
	InterfaceTypes    []string              `yaml:"interface_types"`
	StatCollection    *StatCollectionConfig `yaml:"stat_collection"`
}

// StatCollectionConfig represents interface statistics collection configuration
type StatCollectionConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Interval     int      `yaml:"interval"` // seconds
	IncludeStats []string `yaml:"include_stats"`
}

// NotifyConfig represents notification configuration
type NotifyConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Timeout    time.Duration `yaml:"timeout"`
	RetryCount int           `yaml:"retry_count"`
	RetryDelay time.Duration `yaml:"retry_delay"`
}

// ValidateLogConfig validates logging configuration
func ValidateLogConfig(cfg *LogConfig) error {
	if cfg.Directory == "" {
		return fmt.Errorf("log directory cannot be empty")
	}

	if cfg.MaxSize <= 0 {
		return fmt.Errorf("max_size must be positive")
	}

	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	return nil
}

// ValidateIPVersion validates IP version configuration
func ValidateIPVersion(cfg *IPVersionConfig) error {
	if !cfg.EnableIPv4 && !cfg.EnableIPv6 {
		return fmt.Errorf("at least one IP version must be enabled")
	}
	return nil
}

// ValidateInterface validates interface configuration
func ValidateInterface(cfg *InterfaceConfig) error {
	if len(cfg.InterfaceTypes) == 0 {
		return fmt.Errorf("at least one interface type must be configured")
	}

	for _, ifaceType := range cfg.InterfaceTypes {
		if !isValidInterfaceType(ifaceType) {
			return fmt.Errorf("invalid interface type: %s", ifaceType)
		}
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
