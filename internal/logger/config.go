package logger

import "fmt"

// Config represents logging configuration
type Config struct {
	Directory    string `mapstructure:"directory"`
	File         string `mapstructure:"file"`
	MaxSize      int    `mapstructure:"max_size"` // MB
	MaxBackups   int    `mapstructure:"max_backups"`
	MaxAge       int    `mapstructure:"max_age"` // days
	Compress     bool   `mapstructure:"compress"`
	Level        string `mapstructure:"level"` // debug, info, warn, error
	TimeFormat   string `mapstructure:"time_format"`
	UseLocalTime bool   `mapstructure:"use_local_time"`
}

// Validate validates logging configuration
func (cfg *Config) Validate() error {
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
