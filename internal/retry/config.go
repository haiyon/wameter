package retry

import (
	"encoding/json"
	"errors"
	"time"
)

// Config defines the configuration for the retry mechanism.
type Config struct {
	Enable            bool          `mapstructure:"enable"`              // Enable retry
	InitialAttempts   int           `mapstructure:"initial_attempts"`    // Number of initial fast retries
	InitialInterval   time.Duration `mapstructure:"initial_interval"`    // Interval between initial retries
	MinuteAttempts    int           `mapstructure:"minute_attempts"`     // Number of retries per minute
	MinuteInterval    time.Duration `mapstructure:"minute_interval"`     // Interval between retries in minutes
	HourlyAttempts    int           `mapstructure:"hourly_attempts"`     // Number of retries per hour
	HourlyInterval    time.Duration `mapstructure:"hourly_interval"`     // Interval between retries in hours
	FinalRetryTimeout time.Duration `mapstructure:"final_retry_timeout"` // Timeout for the final retry attempt
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *Config {
	return &Config{
		Enable:            true,
		InitialAttempts:   3,
		InitialInterval:   time.Second,
		MinuteAttempts:    180, // 3 attempts per minute * 60 minutes
		MinuteInterval:    time.Minute,
		HourlyAttempts:    24, // 3 attempts per hour * 8 hours
		HourlyInterval:    time.Hour,
		FinalRetryTimeout: 48 * time.Hour,
	}
}

// Validate validates the retry configuration.
func (cfg *Config) Validate() error {
	if cfg == nil || !cfg.Enable {
		return nil
	}
	if cfg.InitialAttempts <= 0 {
		return errors.New("InitialAttempts must be greater than zero")
	}
	if cfg.InitialInterval < 0 || cfg.MinuteInterval < 0 || cfg.HourlyInterval < 0 || cfg.FinalRetryTimeout < 0 {
		return errors.New("intervals and timeout cannot be negative")
	}
	if cfg.InitialInterval > cfg.FinalRetryTimeout {
		return errors.New("FinalRetryTimeout must be greater than InitialInterval")
	}
	return nil
}

// String returns a JSON string representation of the Config.
func (cfg *Config) String() string {
	data, _ := json.Marshal(cfg)
	return string(data)
}
