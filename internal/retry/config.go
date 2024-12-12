package retry

import (
	"encoding/json"
	"fmt"
	"time"
)

// Stage represents the type of retry stage
type Stage uint8

const (
	// StageInitial represents initial fast retries
	StageInitial Stage = 1 << iota
	// StageMinute represents minute-interval retries
	StageMinute
	// StageHourly represents hourly-interval retries
	StageHourly
	// StageFinal represents final retry attempt
	StageFinal
	// StageAll enables all retry stages
	StageAll = StageInitial | StageMinute | StageHourly | StageFinal
)

// Config defines the configuration for the retry mechanism
type Config struct {
	// Enabled enables the retry mechanism
	Enabled bool `mapstructure:"enabled"`

	// Stage specifies which retry stages are enabled
	Stage Stage `mapstructure:"stage"`

	// InitialAttempts is number of initial fast retries
	InitialAttempts int `mapstructure:"initial_attempts"`

	// InitialInterval is interval between initial retries
	InitialInterval time.Duration `mapstructure:"initial_interval"`

	// MinuteAttempts is number of retries per minute
	MinuteAttempts int `mapstructure:"minute_attempts"`

	// MinuteInterval is interval between retries in minutes
	MinuteInterval time.Duration `mapstructure:"minute_interval"`

	// HourlyAttempts is number of retries per hour
	HourlyAttempts int `mapstructure:"hourly_attempts"`

	// HourlyInterval is interval between retries in hours
	HourlyInterval time.Duration `mapstructure:"hourly_interval"`

	// FinalRetryTimeout is timeout for the final retry attempt
	FinalRetryTimeout time.Duration `mapstructure:"final_retry_timeout"`
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *Config {
	return &Config{
		Enabled:           true,
		Stage:             StageAll,
		InitialAttempts:   3,
		InitialInterval:   time.Second,
		MinuteAttempts:    180, // 3 attempts per minute * 60 minutes
		MinuteInterval:    time.Minute,
		HourlyAttempts:    24, // 3 attempts per hour * 8 hours
		HourlyInterval:    time.Hour,
		FinalRetryTimeout: 48 * time.Hour,
	}
}

// SetDefaults sets default values if not specified
func (cfg *Config) SetDefaults() *Config {
	if cfg != nil && cfg.Enabled {
		if cfg.Stage == 0 {
			cfg.Stage = StageInitial
		}
		if cfg.InitialAttempts <= 0 {
			cfg.InitialAttempts = 3
		}
		if cfg.InitialInterval <= 0 {
			cfg.InitialInterval = time.Second
		}
		if cfg.MinuteAttempts <= 0 {
			cfg.MinuteAttempts = 180
		}
		if cfg.MinuteInterval <= 0 {
			cfg.MinuteInterval = time.Minute
		}
		if cfg.HourlyAttempts <= 0 {
			cfg.HourlyAttempts = 24
		}
		if cfg.HourlyInterval <= 0 {
			cfg.HourlyInterval = time.Hour
		}
		if cfg.FinalRetryTimeout <= 0 {
			cfg.FinalRetryTimeout = 48 * time.Hour
		}
	}

	if cfg == nil {
		return DefaultRetryConfig()
	}
	return cfg
}

// Validate validates the retry configuration
func (cfg *Config) Validate() error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	if cfg.Stage == 0 {
		return fmt.Errorf("no retry stages enabled")
	}

	if cfg.Stage&StageInitial != 0 {
		if cfg.InitialAttempts <= 0 {
			return fmt.Errorf("InitialAttempts must be greater than zero")
		}
		if cfg.InitialInterval <= 0 {
			return fmt.Errorf("InitialInterval must be greater than zero")
		}
	}

	if cfg.Stage&StageMinute != 0 {
		if cfg.MinuteAttempts <= 0 {
			return fmt.Errorf("MinuteAttempts must be greater than zero")
		}
		if cfg.MinuteInterval <= 0 {
			return fmt.Errorf("MinuteInterval must be greater than zero")
		}
	}

	if cfg.Stage&StageHourly != 0 {
		if cfg.HourlyAttempts <= 0 {
			return fmt.Errorf("HourlyAttempts must be greater than zero")
		}
		if cfg.HourlyInterval <= 0 {
			return fmt.Errorf("HourlyInterval must be greater than zero")
		}
	}

	if cfg.Stage&StageFinal != 0 && cfg.FinalRetryTimeout <= 0 {
		return fmt.Errorf("FinalRetryTimeout must be greater than zero")
	}

	return nil
}

// String returns a JSON string representation of the Config.
func (cfg *Config) String() string {
	data, _ := json.Marshal(cfg)
	return string(data)
}
