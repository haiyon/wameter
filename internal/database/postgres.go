package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresDatabase represents PostgreSQL database implementation
type PostgresDatabase struct {
	*Database
}

// NewPostgresDatabase creates new PostgreSQL database instance
func NewPostgresDatabase(dsn string, opts Options, logger *zap.Logger) (Interface, error) {
	// Add parameters
	if !strings.Contains(dsn, "sslmode=") {
		dsn += "?sslmode=disable"
	}

	base, err := newDatabase("postgres", dsn, opts, logger)
	if err != nil {
		return nil, err
	}

	d := &PostgresDatabase{
		Database: base,
	}

	if err := d.init(); err != nil {
		_ = base.Close()
		return nil, fmt.Errorf("failed to initialize PostgreSQL: %w", err)
	}

	return d, nil
}

// init initializes PostgreSQL specific settings
func (d *PostgresDatabase) init() error {
	// Set session variables
	vars := []struct {
		name  string
		value string
	}{
		{"timezone", "'UTC'"},
		{"statement_timeout", "'30s'"},
		{"lock_timeout", "'10s'"},
		{"idle_in_transaction_session_timeout", "'30s'"},
		{"search_path", "'public'"},
		{"work_mem", "'16MB'"},
		{"maintenance_work_mem", "'128MB'"},
		{"random_page_cost", "1.1"},
		{"effective_cache_size", "'1GB'"},
	}

	for _, v := range vars {
		query := fmt.Sprintf("SET %s = %s", v.name, v.value)
		if _, err := d.ExecContext(context.Background(), query); err != nil {
			return fmt.Errorf("failed to set %s: %w", v.name, err)
		}
	}

	return nil
}

// BatchExec implements batch execution for PostgreSQL
func (d *PostgresDatabase) BatchExec(ctx context.Context, query string, args [][]any) error {
	if strings.HasPrefix(strings.ToUpper(query), "INSERT") {
		return d.batchInsert(ctx, query, args)
	}
	return d.Database.BatchExec(ctx, query, args)
}

// batchInsert handles PostgreSQL batch inserts efficiently
func (d *PostgresDatabase) batchInsert(ctx context.Context, query string, args [][]any) error {
	table, cols := extractInsertMetadata(query)
	if table == "" || len(cols) == 0 {
		return fmt.Errorf("invalid INSERT query format")
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	tempTable := fmt.Sprintf("temp_insert_%d", time.Now().UnixNano())
	createTempQuery := fmt.Sprintf(
		"CREATE TEMP TABLE %s (LIKE %s INCLUDING ALL) ON COMMIT DROP",
		tempTable, table)

	if _, err := tx.ExecContext(ctx, createTempQuery); err != nil {
		return fmt.Errorf("failed to create temp table: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx,
		fmt.Sprintf("COPY %s (%s) FROM STDIN WITH (FORMAT binary)", tempTable, strings.Join(cols, ",")))
	if err != nil {
		return fmt.Errorf("failed to prepare COPY statement: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, arg := range args {
		if _, err := stmt.ExecContext(ctx, arg...); err != nil {
			return fmt.Errorf("COPY failed: %w", err)
		}
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s SELECT * FROM %s",
		table, tempTable)

	if _, err := tx.ExecContext(ctx, insertQuery); err != nil {
		return fmt.Errorf("failed to insert from temp table: %w", err)
	}

	return tx.Commit()
}

// WithTransaction overrides default implementation for PostgreSQL
func (d *PostgresDatabase) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
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

// Cleanup implements data cleanup for PostgreSQL
func (d *PostgresDatabase) Cleanup(ctx context.Context, before time.Time) error {
	query := `
        WITH deleted AS (
            DELETE FROM metrics
            WHERE timestamp < $1
            RETURNING *
        )
        SELECT count(*) FROM deleted`

	var count int64
	err := d.QueryRowContext(ctx, query, before).Scan(&count)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	if count > 0 {
		if _, err := d.ExecContext(ctx, "VACUUM ANALYZE metrics"); err != nil {
			d.logger.Warn("Failed to vacuum table after cleanup",
				zap.Error(err))
		}
	}

	return nil
}

// extractInsertMetadata extracts table name and columns from INSERT query
func extractInsertMetadata(query string) (string, []string) {
	query = strings.TrimSpace(strings.ToUpper(query))

	if !strings.HasPrefix(query, "INSERT INTO") {
		return "", nil
	}

	parts := strings.SplitN(query[12:], "(", 2)
	if len(parts) != 2 {
		return "", nil
	}

	table := strings.TrimSpace(parts[0])
	colsPart := parts[1][:strings.Index(parts[1], ")")]

	cols := strings.Split(colsPart, ",")
	for i := range cols {
		cols[i] = strings.TrimSpace(cols[i])
	}

	return table, cols
}
