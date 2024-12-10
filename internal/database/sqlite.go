package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// SQLiteDatabase represents SQLite specific implementation
type SQLiteDatabase struct {
	*Database
	path string
}

// NewSQLiteDatabase creates new SQLite database instance
func NewSQLiteDatabase(dsn string, opts Options, logger *zap.Logger) (Interface, error) {
	// Ensure the database directory exists
	if err := ensureDBDir(dsn); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Add SQLite parameters
	dsn = addSQLiteParams(dsn, opts)

	base, err := newDatabase("sqlite3", dsn, opts, logger)
	if err != nil {
		return nil, err
	}

	d := &SQLiteDatabase{
		Database: base,
		path:     dsn,
	}

	if err := d.init(); err != nil {
		_ = base.Close()
		return nil, fmt.Errorf("failed to initialize SQLite: %w", err)
	}

	return d, nil
}

// init initializes SQLite specific settings
func (d *SQLiteDatabase) init() error {
	pragmas := []struct {
		name  string
		value string
	}{
		{"journal_mode", "WAL"},
		{"synchronous", "NORMAL"},
		{"cache_size", "-2000"},
		{"foreign_keys", "ON"},
		{"temp_store", "MEMORY"},
		{"mmap_size", "268435456"},
		{"busy_timeout", "5000"},
		{"auto_vacuum", "INCREMENTAL"},
		{"page_size", "4096"},
		{"secure_delete", "OFF"},
		{"busy_timeout", "5000"},
	}

	for _, pragma := range pragmas {
		query := fmt.Sprintf("PRAGMA %s = %s", pragma.name, pragma.value)
		if _, err := d.ExecContext(context.Background(), query); err != nil {
			return fmt.Errorf("failed to set %s: %w", pragma.name, err)
		}
	}

	return nil
}

// BatchExec implements batch execution for SQLite
func (d *SQLiteDatabase) BatchExec(ctx context.Context, query string, args [][]any) error {
	tx, err := d.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	defer func() {
		_ = stmt.Close()
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, arg := range args {
		if _, err = stmt.ExecContext(ctx, arg...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// WithTransaction overrides default implementation with SQLite specific optimizations
func (d *SQLiteDatabase) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

// Backup creates a backup of the database
func (d *SQLiteDatabase) Backup(dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	query := fmt.Sprintf("VACUUM INTO '%s'", dst)
	if _, err := d.ExecContext(context.Background(), query); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	return nil
}

// Optimize optimizes the database
func (d *SQLiteDatabase) Optimize() error {
	optimizations := []string{
		"PRAGMA optimize",
		"ANALYZE",
		"VACUUM",
	}

	for _, opt := range optimizations {
		if _, err := d.ExecContext(context.Background(), opt); err != nil {
			return fmt.Errorf("failed to run %s: %w", opt, err)
		}
	}

	return nil
}

// Cleanup implements data cleanup for SQLite
func (d *SQLiteDatabase) Cleanup(ctx context.Context, before time.Time) error {
	batchSize := 500
	var totalDeleted int64

	for {
		result, err := d.ExecContext(ctx,
			"DELETE FROM metrics WHERE timestamp < ? LIMIT ?",
			before, batchSize)
		if err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}

		totalDeleted += affected
		if affected < int64(batchSize) {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if totalDeleted > 0 {
		if err := d.vacuum(ctx); err != nil {
			d.logger.Warn("Failed to vacuum database after cleanup",
				zap.Error(err))
		}
	}

	return nil
}

// vacuum performs database vacuuming
func (d *SQLiteDatabase) vacuum(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, "PRAGMA incremental_vacuum"); err != nil {
		return fmt.Errorf("incremental vacuum failed: %w", err)
	}

	info, err := os.Stat(d.path)
	if err != nil {
		return err
	}

	if info.Size() > 1024*1024*1024 {
		if _, err := d.ExecContext(ctx, "VACUUM"); err != nil {
			return fmt.Errorf("full vacuum failed: %w", err)
		}
	}

	return nil
}

// ensureDBDir ensures database directory exists
func ensureDBDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// addSQLiteParams adds SQLite specific connection parameters
func addSQLiteParams(dsn string, opts Options) string {
	params := []string{
		"_busy_timeout=5000",
		"_journal_mode=WAL",
		"_synchronous=NORMAL",
		fmt.Sprintf("_cache_size=-%d", opts.MaxOpenConns*200),
		"_foreign_keys=1",
		"_temp_store=MEMORY",
	}

	query := "?" + strings.Join(params, "&")
	if strings.Contains(dsn, "?") {
		query = "&" + strings.Join(params, "&")
	}

	return dsn + query
}
