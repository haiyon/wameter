package service

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wameter/internal/server/config"
	"wameter/internal/server/notify"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// ConfigService represents configuration management service interface
type ConfigService interface {
	GetConfig() *config.Config
	UpdateConfig(ctx context.Context, cfg *config.Config) error
	ReloadConfig(ctx context.Context) error
	ValidateConfig(cfg *config.Config) error
	GetConfigHistory(ctx context.Context) ([]types.ConfigChange, error)
}

// _ implements ConfigService
var _ ConfigService = (*Service)(nil)

// configManager handles configuration management
type configManager struct {
	current *config.Config
	history []types.ConfigChange
	mu      sync.RWMutex
	logger  *zap.Logger
}

// NewConfigManager creates new configuration manager
func NewConfigManager(cfg *config.Config, logger *zap.Logger) *configManager {
	return &configManager{
		current: cfg,
		history: make([]types.ConfigChange, 0),
		logger:  logger,
	}
}

// GetConfig returns current configuration
func (s *Service) GetConfig() *config.Config {
	s.configMgr.mu.RLock()
	defer s.configMgr.mu.RUnlock()
	return s.configMgr.current
}

// UpdateConfig updates configuration
func (s *Service) UpdateConfig(ctx context.Context, newCfg *config.Config) error {
	// First validate new configuration
	if err := s.ValidateConfig(newCfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	s.configMgr.mu.Lock()
	defer s.configMgr.mu.Unlock()

	// Detect changes
	changes := detectConfigChanges(s.configMgr.current, newCfg)
	if len(changes) == 0 {
		return nil // No changes detected
	}

	// Create change record
	change := types.ConfigChange{
		Timestamp: time.Now(),
		Changes:   changes,
	}

	// Apply changes to components
	if err := s.applyConfigChanges(ctx, newCfg, changes); err != nil {
		return fmt.Errorf("failed to apply configuration changes: %w", err)
	}

	// Update current configuration
	s.configMgr.current = newCfg
	s.configMgr.history = append(s.configMgr.history, change)

	s.logger.Info("Configuration updated",
		zap.Int("changes", len(changes)))

	return nil
}

// ReloadConfig reloads configuration from file
func (s *Service) ReloadConfig(ctx context.Context) error {
	// Load configuration from file
	newCfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Update configuration
	return s.UpdateConfig(ctx, newCfg)
}

// ValidateConfig validates configuration
func (s *Service) ValidateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate server configuration
	if err := cfg.Server.Validate(); err != nil {
		return err
	}

	// Validate database configuration
	if err := cfg.Database.Validate(); err != nil {
		return err
	}

	// Validate notification configuration
	if err := cfg.Notify.Validate(); err != nil {
		return err
	}

	return nil
}

// GetConfigHistory returns configuration change history
func (s *Service) GetConfigHistory(ctx context.Context) ([]types.ConfigChange, error) {
	s.configMgr.mu.RLock()
	defer s.configMgr.mu.RUnlock()

	history := make([]types.ConfigChange, len(s.configMgr.history))
	copy(history, s.configMgr.history)

	return history, nil
}

// Internal helper functions

// detectConfigChanges detects changes between configurations
func detectConfigChanges(old, new *config.Config) []types.ConfigModification {
	var changes []types.ConfigModification

	// Server changes
	if old.Server.Address != new.Server.Address {
		changes = append(changes, types.ConfigModification{
			Path:     "server.address",
			OldValue: old.Server.Address,
			NewValue: new.Server.Address,
		})
	}

	// Database changes
	if old.Database.MaxConnections != new.Database.MaxConnections {
		changes = append(changes, types.ConfigModification{
			Path:     "database.max_connections",
			OldValue: old.Database.MaxConnections,
			NewValue: new.Database.MaxConnections,
		})
	}

	// Notification changes
	if old.Notify.Enabled != new.Notify.Enabled {
		changes = append(changes, types.ConfigModification{
			Path:     "notify.enabled",
			OldValue: old.Notify.Enabled,
			NewValue: new.Notify.Enabled,
		})
	}

	return changes
}

// applyConfigChanges applies configuration changes to components
func (s *Service) applyConfigChanges(_ context.Context, cfg *config.Config, changes []types.ConfigModification) error {
	for _, change := range changes {
		switch change.Path {
		case "database.max_connections":
			if err := s.updateDatabaseConnections(cfg.Database.MaxConnections); err != nil {
				return err
			}
		case "notify.enabled":
			if err := s.updateNotifierStatus(cfg.Notify.Enabled); err != nil {
				return err
			}
			// Add more change handlers as needed
		}
	}

	return nil
}

// updateDatabaseConnections updates database connection pool
func (s *Service) updateDatabaseConnections(_ int) error {
	if s.db != nil {
		// Update database connection pool
		// s.db.SetMaxOpenConns(maxConns)
	}
	return nil
}

// updateNotifierStatus updates notifier status
func (s *Service) updateNotifierStatus(enabled bool) error {
	if enabled && s.notifier == nil {
		// Initialize notifier
		notifier, err := notify.NewManager(s.configMgr.current.Notify, s.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize notifier: %w", err)
		}
		s.notifier = notifier
	} else if !enabled && s.notifier != nil {
		// Stop notifier
		if err := s.notifier.Stop(); err != nil {
			return fmt.Errorf("failed to stop notifier: %w", err)
		}
		s.notifier = nil
	}
	return nil
}
