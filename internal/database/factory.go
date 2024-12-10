package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wameter/internal/database/migration"
	"wameter/internal/server/config"

	"go.uber.org/zap"
)

// New creates new database instance based on configuration
func New(cfg *config.DatabaseConfig, logger *zap.Logger) (Interface, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid database config: %w", err)
	}

	// Create a new database instance
	db, err := newInstance(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations
	if cfg.AutoMigrate {
		if err := runMigrations(cfg, logger); err != nil {
			logger.Error("Failed to run migrations", zap.Error(err))
			return nil, err
		}
	}

	return db, nil
}

// newInstance creates new database instance based on configuration
func newInstance(cfg *config.DatabaseConfig, logger *zap.Logger) (Interface, error) {
	// Set options
	opts := Options{
		MaxOpenConns:       cfg.MaxConnections,
		MaxIdleConns:       cfg.MaxIdleConns,
		ConnMaxLifetime:    cfg.ConnMaxLifetime,
		ConnMaxIdleTime:    cfg.ConnMaxLifetime,
		QueryTimeout:       cfg.QueryTimeout,
		MaxBatchSize:       cfg.MaxBatchSize,
		StatementCache:     cfg.StatementCache,
		EnableMetrics:      cfg.EnableMetrics,
		EnablePruning:      cfg.EnablePruning,
		PruneInterval:      cfg.PruneInterval,
		RetentionPeriod:    cfg.MetricsRetention,
		SlowQueryThreshold: cfg.SlowQueryTime,
	}

	// Create instance
	switch cfg.Driver {
	case "sqlite":
		return NewSQLiteDatabase(cfg.DSN, opts, logger)
	case "mysql":
		return NewMySQLDatabase(cfg.DSN, opts, logger)
	case "postgres":
		return NewPostgresDatabase(cfg.DSN, opts, logger)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

// runMigrations runs database migrations based on the configuration
func runMigrations(cfg *config.DatabaseConfig, logger *zap.Logger) error {
	// Create a new database connection for migrations
	db, err := newInstance(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create database connection for migrations: %w", err)
	}

	defer func() {
		_ = db.Close()
	}()

	// Verify migrations path
	migrationsPath := cfg.MigrationsPath
	if _, err := os.Stat(migrationsPath); err != nil {
		return fmt.Errorf("migrations path %s does not exist: %w", migrationsPath, err)
	}

	// Ensure driver-specific migrations exist
	driverPath := filepath.Join(migrationsPath, cfg.Driver)
	if _, err := os.Stat(driverPath); err != nil {
		return fmt.Errorf("driver-specific migrations path %s does not exist: %w", driverPath, err)
	}
	cfg.MigrationsPath = driverPath

	// Create migrator instance
	migrator, err := migration.NewMigrator(db.Unwrap(), cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	defer func() {
		if err := migrator.Close(); err != nil {
			logger.Error("Failed to close migrator", zap.Error(err))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run migrations
	if cfg.RollbackSteps > 0 {
		logger.Info("Rolling back migrations", zap.Int("steps", cfg.RollbackSteps))
		if err := migrator.RollbackMigrations(ctx, cfg.RollbackSteps); err != nil {
			return fmt.Errorf("failed to rollback migrations: %w", err)
		}
	} else if cfg.TargetVersion > 0 {
		logger.Info("Migrating to target version", zap.Int("target_version", cfg.TargetVersion))
		if err := migrator.MigrateToVersion(ctx, uint(cfg.TargetVersion)); err != nil {
			return fmt.Errorf("failed to migrate to target version: %w", err)
		}
	} else {
		logger.Info("Running migrations to latest version")
		if err := migrator.RunMigrations(ctx); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	return nil
}
