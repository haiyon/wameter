package config

import (
	"fmt"
	"time"
	"wameter/internal/config"

	"github.com/spf13/viper"
)

// Config represents the complete server configuration
type Config struct {
	Server   ServerConfig         `mapstructure:"server"`
	Database DatabaseConfig       `mapstructure:"database"`
	Notify   *config.NotifyConfig `mapstructure:"notify"`
	API      APIConfig            `mapstructure:"api"`
	Log      *config.LogConfig    `mapstructure:"log"`
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

// ServerConfig represents the server configuration
type ServerConfig struct {
	Address      string        `mapstructure:"address"`
	MetricsPath  string        `mapstructure:"metrics_path"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	TLS          TLSConfig     `mapstructure:"tls"`
}

// Validate server configuration
func (cfg *ServerConfig) Validate() error {
	if cfg.Address == "" {
		return fmt.Errorf("server address is required")
	}
	return nil
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

// Validate TLS configuration
func (cfg *TLSConfig) Validate() error {
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return fmt.Errorf("TLS cert and key files are required")
	}
	return nil
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

// Validate API configuration
func (cfg *APIConfig) Validate() error {
	if cfg.Auth.Enabled {
		if err := cfg.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid auth config: %w", err)
		}
	}
	return nil
}

// AuthConfig represents the authentication configuration
type AuthConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Type         string        `mapstructure:"type"` // jwt, apikey, basic
	JWTSecret    string        `mapstructure:"jwt_secret"`
	JWTDuration  time.Duration `mapstructure:"jwt_duration"`
	AllowedUsers []string      `mapstructure:"allowed_users"`
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

// CORSConfig represents the CORS configuration
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	MaxAge           int      `mapstructure:"max_age"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

// Validate CORS configuration
func (cfg *CORSConfig) Validate() error {
	if len(cfg.AllowedOrigins) == 0 {
		return fmt.Errorf("allowed origins list is required")
	}
	return nil
}

// RateLimitConfig represents the rate limiting configuration
type RateLimitConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Requests int           `mapstructure:"requests"`
	Window   time.Duration `mapstructure:"window"`
	Strategy string        `mapstructure:"strategy"` // token, leaky, sliding
}

// Validate rate limiting configuration
func (cfg *RateLimitConfig) Validate() error {
	switch cfg.Strategy {
	case "token", "leaky", "sliding":
	default:
		return fmt.Errorf("unsupported rate limit strategy: %s", cfg.Strategy)
	}
	return nil
}

// MetricsConfig represents the metrics configuration
type MetricsConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	Prometheus  bool   `mapstructure:"prometheus"`
	StatsdAddr  string `mapstructure:"statsd_addr"`
	ServiceName string `mapstructure:"service_name"`
}

// Validate metrics configuration
func (cfg *MetricsConfig) Validate() error {
	if cfg.Path == "" {
		return fmt.Errorf("metrics path is required")
	}
	return nil
}

// DocsConfig represents the documentation configuration
type DocsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Title   string `mapstructure:"title"`
	Version string `mapstructure:"version"`
}

// Validate docs configuration
func (cfg *DocsConfig) Validate() error {
	if cfg.Path == "" {
		return fmt.Errorf("docs path is required")
	}
	return nil
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

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration
func setDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8080"
	}

	if cfg.Server.MetricsPath == "" {
		cfg.Server.MetricsPath = "/metrics"
	}

	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}

	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}

	if cfg.API.RateLimit.Window == 0 {
		cfg.API.RateLimit.Window = time.Minute
	}

	if cfg.API.RateLimit.Requests == 0 {
		cfg.API.RateLimit.Requests = 60
	}

	if cfg.API.CORS.MaxAge == 0 {
		cfg.API.CORS.MaxAge = 86400
	}

	// Set default allowed methods for CORS
	if len(cfg.API.CORS.AllowedMethods) == 0 {
		cfg.API.CORS.AllowedMethods = []string{
			"GET", "POST", "PUT", "DELETE", "OPTIONS",
		}
	}

	// Set default allowed headers for CORS
	if len(cfg.API.CORS.AllowedHeaders) == 0 {
		cfg.API.CORS.AllowedHeaders = []string{
			"Content-Type", "Authorization", "X-Request-ID",
		}
	}
}
