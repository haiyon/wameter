package config

import (
	"fmt"
	"time"
	commonCfg "wameter/internal/config"

	"github.com/spf13/viper"
)

// Config represents the complete server configuration
type Config struct {
	Server   ServerConfig            `mapstructure:"server"`
	Database Database                `mapstructure:"database"`
	Notify   *commonCfg.NotifyConfig `mapstructure:"notify"`
	API      APIConfig               `mapstructure:"api"`
	Log      LogConfig               `mapstructure:"log"`
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
	if err := config.Validate(); err != nil {
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

// Validate validates the configuration
func (cfg *Config) Validate() error {
	// Validate database configuration
	if err := cfg.Database.Validate(); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}

	// Validate TLS configuration
	if cfg.Server.TLS.Enabled {
		if err := cfg.Server.TLS.Validate(); err != nil {
			return fmt.Errorf("invalid TLS config: %w", err)
		}
	}

	// Validate notification configuration
	if err := cfg.Notify.Validate(); err != nil {
		return fmt.Errorf("invalid notification config: %w", err)
	}

	// Validate API configuration
	if err := cfg.API.Validate(); err != nil {
		return fmt.Errorf("invalid API config: %w", err)
	}

	return nil
}

// Validate TLS configuration
func (cfg *TLSConfig) Validate() error {
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return fmt.Errorf("TLS cert and key files are required")
	}
	return nil
}

// Validate API configuration
func (cfg *APIConfig) Validate() error {
	if cfg.Auth.Enabled {
		if err := cfg.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid auth config: %w", err)
		}
	}
	return nil
}

// Validate auth configuration
func (cfg *AuthConfig) Validate() error {
	switch cfg.Type {
	case "jwt":
		if cfg.JWTSecret == "" {
			return fmt.Errorf("JWT secret is required")
		}
	case "apikey", "basic":
		if len(cfg.AllowedUsers) == 0 {
			return fmt.Errorf("allowed users list is required")
		}
	default:
		return fmt.Errorf("unsupported auth type: %s", cfg.Type)
	}
	return nil
}
