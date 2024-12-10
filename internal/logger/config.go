package logger

import "fmt"

// Config represents logging configuration
type Config struct {
	Directory    string `yaml:"directory"`
	File         string `yaml:"file"`
	MaxSize      int    `yaml:"max_size"` // MB
	MaxBackups   int    `yaml:"max_backups"`
	MaxAge       int    `yaml:"max_age"` // days
	Compress     bool   `yaml:"compress"`
	Level        string `yaml:"level"` // debug, info, warn, error
	TimeFormat   string `yaml:"time_format"`
	UseLocalTime bool   `yaml:"use_local_time"`
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
