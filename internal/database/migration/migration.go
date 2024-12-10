package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"wameter/internal/server/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

// Migrator handles database migrations
type Migrator struct {
	config  *config.DatabaseConfig
	migrate *migrate.Migrate
	logger  *zap.Logger
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB, cfg *config.DatabaseConfig, logger *zap.Logger) (*Migrator, error) {
	if _, err := os.Stat(cfg.MigrationsPath); err != nil {
		return nil, fmt.Errorf("migrations path %s does not exist: %w", cfg.MigrationsPath, err)
	}

	var (
		instance *migrate.Migrate
		err      error
		driver   database.Driver
	)

	switch cfg.Driver {
	case "sqlite":
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create sqlite driver: %w", err)
		}
		instance, err = migrate.NewWithDatabaseInstance("file://"+cfg.MigrationsPath, "sqlite3", driver)

	case "mysql":
		driver, err = mysql.WithInstance(db, &mysql.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create mysql driver: %w", err)
		}
		instance, err = migrate.NewWithDatabaseInstance("file://"+cfg.MigrationsPath, "mysql", driver)

	case "postgres":
		driver, err = postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres driver: %w", err)
		}
		instance, err = migrate.NewWithDatabaseInstance("file://"+cfg.MigrationsPath, "postgres", driver)

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create migrator instance: %w", err)
	}

	return &Migrator{
		config:  cfg,
		migrate: instance,
		logger:  logger,
	}, nil
}

// RunMigrations executes pending migrations
func (m *Migrator) RunMigrations(ctx context.Context) error {
	if m.migrate == nil {
		return errors.New("migrator not properly initialized")
	}

	m.logger.Info("Starting migrations...")
	errChan := make(chan error, 1)

	go func() {
		if err := m.migrate.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			errChan <- fmt.Errorf("migration failed: %w", err)
			return
		}
		errChan <- nil
	}()

	select {
	case <-ctx.Done():
		m.logger.Warn("Migration cancelled by context")
		return fmt.Errorf("migration cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			m.logger.Error("Migration failed", zap.Error(err))
			return err
		}
		m.logger.Info("Migrations completed successfully")
		return nil
	}
}

// RollbackMigrations rolls back the last `steps` migrations
func (m *Migrator) RollbackMigrations(ctx context.Context, steps int) error {
	if m.migrate == nil {
		return errors.New("migrator not properly initialized")
	}

	errChan := make(chan error, 1)

	go func() {
		if err := m.migrate.Steps(-steps); err != nil {
			errChan <- fmt.Errorf("rollback failed: %w", err)
			return
		}
		errChan <- nil
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("rollback cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			m.logger.Error("Rollback failed", zap.Error(err))
			return err
		}
		return nil
	}
}

// MigrateToVersion migrates to a specific version
func (m *Migrator) MigrateToVersion(ctx context.Context, version uint) error {
	if m.migrate == nil {
		return errors.New("migrator not properly initialized")
	}

	errChan := make(chan error, 1)

	go func() {
		if err := m.migrate.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			errChan <- fmt.Errorf("migration to version %d failed: %w", version, err)
			return
		}
		errChan <- nil
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("migration to version cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			m.logger.Error("Migration to version failed", zap.Error(err))
			return err
		}
		return nil
	}
}

// GetVersion returns the current migration version
func (m *Migrator) GetVersion() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}

// Close releases resources
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	if sourceErr == nil && dbErr == nil {
		return nil
	}

	var errMsg string
	if sourceErr != nil {
		errMsg = fmt.Sprintf("source error: %v", sourceErr)
	}
	if dbErr != nil {
		if errMsg != "" {
			errMsg += "; "
		}
		errMsg += fmt.Sprintf("database error: %v", dbErr)
	}

	return fmt.Errorf("failed to close migrator: %s", errMsg)
}
