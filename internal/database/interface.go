package database

import (
	"context"
	"database/sql"
	"time"
)

// Interface defines the database interface
type Interface interface {
	// Basic operations

	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)

	// Transaction operations

	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error

	// Batch operations

	BatchExec(ctx context.Context, query string, args [][]any) error
	BatchQuery(ctx context.Context, query string, args [][]any, fn func(*sql.Rows) error) error

	// Statement management

	CacheStmt(query string, stmt *sql.Stmt)
	GetCachedStmt(query string) *sql.Stmt
	ClearStmtCache()

	// Maintenance operations

	Ping(ctx context.Context) error
	Close() error
	Stats() Stats
	Driver() string

	// Data maintenance

	Cleanup(ctx context.Context, before time.Time) error
	RunPruning(ctx context.Context) error
	StopPruning() error
	Unwrap() (*sql.DB, error)
}
