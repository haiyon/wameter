package logger

import "fmt"

// Config represents logging configuration
type Config struct {
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"` // MB
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"` // days
	Compress   bool   `mapstructure:"compress"`
	Level      string `mapstructure:"level"` // debug, info, warn, error
}

// Validate validates logging configuration
func (cfg *Config) Validate() error {
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

// DefaultConfig returns the default logging configuration
func DefaultConfig() *Config {
	return &Config{
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Level:      "info",
	}
}

// SetDefaults sets default values if not specified
func (cfg *Config) SetDefaults() *Config {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 100
	}

	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 7
	}

	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 30
	}

	if cfg.Level == "" {
		cfg.Level = "info"
	}

	return cfg
}
