package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// MySQLDatabase represents MySQL specific implementation
type MySQLDatabase struct {
	*Database
}

// NewMySQLDatabase creates new MySQL database instance
func NewMySQLDatabase(dsn string, opts Options, logger *zap.Logger) (Interface, error) {
	// Add parameters
	params := []string{
		"charset=utf8mb4",
		"interpolateParams=true",
	}

	if !strings.Contains(dsn, "parseTime=true") {
		params = append(params, "parseTime=true")
	}

	// Append params to DSN
	queryStart := "?"
	if strings.Contains(dsn, "?") {
		queryStart = "&"
	}
	dsn += queryStart + strings.Join(params, "&")

	base, err := newDatabase("mysql", dsn, opts, logger)
	if err != nil {
		return nil, err
	}

	d := &MySQLDatabase{
		Database: base,
	}

	if err := d.init(); err != nil {
		_ = base.Close()
		return nil, fmt.Errorf("failed to initialize MySQL: %w", err)
	}

	return d, nil
}

// init initializes MySQL specific settings
func (d *MySQLDatabase) init() error {
	// Set session variables
	vars := []struct {
		name  string
		value string
	}{
		{"sql_mode", "'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION'"},
		{"time_zone", "'+00:00'"},
		{"wait_timeout", "28800"},
		{"interactive_timeout", "28800"},
		{"net_read_timeout", "30"},
		{"net_write_timeout", "30"},
		{"innodb_lock_wait_timeout", "20"},
		{"long_query_time", "1"},
		{"innodb_lock_wait_timeout", "50"},
		// {"max_allowed_packet", "16777216"},
	}

	for _, v := range vars {
		query := fmt.Sprintf("SET SESSION %s = %s", v.name, v.value)
		if _, err := d.ExecContext(context.Background(), query); err != nil {
			return fmt.Errorf("failed to set %s: %w", v.name, err)
		}
	}

	return nil
}

// BatchExec implements batch execution for MySQL
func (d *MySQLDatabase) BatchExec(ctx context.Context, query string, args [][]any) error {
	if strings.HasPrefix(strings.ToUpper(query), "INSERT") {
		return d.batchInsert(ctx, query, args)
	}
	return d.Database.BatchExec(ctx, query, args)
}

// batchInsert handles MySQL batch inserts efficiently
func (d *MySQLDatabase) batchInsert(ctx context.Context, query string, args [][]any) error {
	idx := strings.Index(strings.ToUpper(query), "VALUES")
	if idx == -1 {
		return fmt.Errorf("invalid INSERT query format")
	}

	baseQuery := query[:idx]
	valueStr := query[idx+6:]
	valueStr = strings.TrimSpace(valueStr)
	valueStr = strings.Trim(valueStr, "()")

	var allArgs []any
	placeholders := make([]string, len(args))

	for i, arg := range args {
		placeholders[i] = "(" + strings.Repeat("?,", len(arg)-1) + "?)"
		allArgs = append(allArgs, arg...)
	}

	fullQuery := baseQuery + " VALUES " + strings.Join(placeholders, ",")
	_, err := d.ExecContext(ctx, fullQuery, allArgs...)
	return err
}

// WithTransaction overrides default implementation for MySQL
func (d *MySQLDatabase) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead, // Use default isolation
	})
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

// Cleanup implements data cleanup for MySQL
func (d *MySQLDatabase) Cleanup(ctx context.Context, before time.Time) error {
	batchSize := 1000
	totalDeleted := int64(0)

	for {
		query := `DELETE FROM metrics WHERE timestamp < ? LIMIT ?`
		result, err := d.ExecContext(ctx, query, before, batchSize)
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

		time.Sleep(100 * time.Millisecond)
	}

	if totalDeleted > 0 {
		query := "OPTIMIZE TABLE metrics"
		if _, err := d.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to optimize table: %w", err)
		}
	}

	return nil
}
