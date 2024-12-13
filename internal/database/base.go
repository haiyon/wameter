package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Database represents the base database implementation
type Database struct {
	db          *sql.DB
	driver      string
	logger      *zap.Logger
	opts        Options
	metrics     *metrics
	pruneCtx    context.Context
	pruneCancel context.CancelFunc
	stmtCache   sync.Map
	mu          sync.RWMutex
}

// metrics represents database metrics
type metrics struct {
	queryCount    int64
	queryErrors   int64
	slowQueries   int64
	queryTime     int64
	cacheHits     int64
	cacheMisses   int64
	lastError     error
	lastErrorTime time.Time
}

// newDatabase creates new base database instance
func newDatabase(driver, dsn string, opts Options, logger *zap.Logger) (*Database, error) {
	// Set default options
	if opts.MaxOpenConns <= 0 {
		opts.MaxOpenConns = 25
	}
	if opts.MaxIdleConns <= 0 {
		opts.MaxIdleConns = 5
	}
	if opts.ConnMaxLifetime <= 0 {
		opts.ConnMaxLifetime = 3600 * time.Second
	}
	if opts.QueryTimeout <= 0 {
		opts.QueryTimeout = 60 * time.Second
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(opts.MaxOpenConns)
	db.SetMaxIdleConns(opts.MaxIdleConns)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	db.SetConnMaxIdleTime(opts.ConnMaxIdleTime)

	// Create pruning context
	pruneCtx, pruneCancel := context.WithCancel(context.Background())

	d := &Database{
		db:          db,
		driver:      driver,
		logger:      logger,
		opts:        opts,
		metrics:     &metrics{},
		pruneCtx:    pruneCtx,
		pruneCancel: pruneCancel,
	}

	// Start pruning if enabled
	if opts.EnablePruning {
		go d.pruneLoop()
	}

	// Health check
	go d.healthCheck()

	return d, nil
}

// ExecContext executes query and returns result
func (d *Database) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	// Add timeout if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.opts.QueryTimeout)
		defer cancel()
	}

	start := time.Now()
	result, err := d.db.ExecContext(ctx, query, args...)
	d.recordMetrics(start, err)

	return result, err
}

// QueryContext executes query and returns rows
func (d *Database) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	// Add timeout if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.opts.QueryTimeout)
		defer cancel()
	}

	start := time.Now()
	rows, err := d.db.QueryContext(ctx, query, args...)
	d.recordMetrics(start, err)

	return rows, err
}

// QueryRowContext executes query and returns row
func (d *Database) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	// Add timeout if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.opts.QueryTimeout)
		defer cancel()
	}

	start := time.Now()
	row := d.db.QueryRowContext(ctx, query, args...)
	d.recordMetrics(start, nil)
	return row
}

// PrepareContext prepares statement and returns it
func (d *Database) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	// Check statement cache first
	if d.opts.StatementCache {
		if stmt, ok := d.stmtCache.Load(query); ok {
			atomic.AddInt64(&d.metrics.cacheHits, 1)
			return stmt.(*sql.Stmt), nil
		}
		atomic.AddInt64(&d.metrics.cacheMisses, 1)
	}

	// Add timeout if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.opts.QueryTimeout)
		defer cancel()
	}

	stmt, err := d.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}

	// Cache statement if enabled
	if d.opts.StatementCache {
		d.stmtCache.Store(query, stmt)
	}

	return stmt, nil
}

// BeginTx starts a transaction
func (d *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	// Add timeout if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.opts.QueryTimeout)
		defer cancel()
	}

	return d.db.BeginTx(ctx, opts)
}

// WithTransaction executes a transaction
func (d *Database) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				d.logger.Error("Transaction rollback failed during panic",
					zap.Error(rbErr))
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

// BatchExec executes a batch of queries
func (d *Database) BatchExec(ctx context.Context, query string, args [][]any) error {
	return d.WithTransaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}

		defer func(stmt *sql.Stmt) {
			_ = stmt.Close()
		}(stmt)

		for _, arg := range args {
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, arg...); err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchQuery executes a batch of queries and processes results
func (d *Database) BatchQuery(ctx context.Context, query string, args [][]any, fn func(*sql.Rows) error) error {
	return d.WithTransaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}

		defer func(stmt *sql.Stmt) {
			_ = stmt.Close()
		}(stmt)

		for _, arg := range args {
			rows, err := stmt.QueryContext(ctx, arg...)
			if err != nil {
				return err
			}

			if err := fn(rows); err != nil {
				_ = rows.Close()
				return err
			}
			_ = rows.Close()
		}
		return nil
	})
}

// CacheStmt caches a prepared statement
func (d *Database) CacheStmt(query string, stmt *sql.Stmt) {
	if d.opts.StatementCache {
		d.stmtCache.Store(query, stmt)
	}
}

// GetCachedStmt retrieves a cached statement
func (d *Database) GetCachedStmt(query string) *sql.Stmt {
	if !d.opts.StatementCache {
		return nil
	}
	if stmt, ok := d.stmtCache.Load(query); ok {
		return stmt.(*sql.Stmt)
	}
	return nil
}

// ClearStmtCache clears the statement cache
func (d *Database) ClearStmtCache() {
	d.stmtCache.Range(func(key, value any) bool {
		stmt := value.(*sql.Stmt)
		if err := stmt.Close(); err != nil {
			d.logger.Error("Failed to close prepared statement", zap.Error(err))
		}
		d.stmtCache.Delete(key)
		return true
	})
}

// Ping pings the database
func (d *Database) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Close closes the database connection and cleans up resources
func (d *Database) Close() error {
	// Stop pruning
	if d.pruneCancel != nil {
		d.pruneCancel()
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	// Close prepared statements
	d.stmtCache.Range(func(key, value any) bool {
		stmt := value.(*sql.Stmt)
		if err := stmt.Close(); err != nil {
			d.logger.Error("Failed to close prepared statement", zap.Error(err))
		}
		d.stmtCache.Delete(key)
		return true
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- d.db.Close()
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("database close timeout")
	}
}

// Stats returns database statistics
func (d *Database) Stats() Stats {
	dbStats := d.db.Stats()
	return Stats{
		OpenConnections: dbStats.OpenConnections,
		InUse:           dbStats.InUse,
		Idle:            dbStats.Idle,
		WaitCount:       dbStats.WaitCount,
		WaitDuration:    dbStats.WaitDuration,
		QueryCount:      atomic.LoadInt64(&d.metrics.queryCount),
		QueryErrors:     atomic.LoadInt64(&d.metrics.queryErrors),
		SlowQueries:     atomic.LoadInt64(&d.metrics.slowQueries),
		AvgQueryTime:    time.Duration(atomic.LoadInt64(&d.metrics.queryTime) / atomic.LoadInt64(&d.metrics.queryCount)),
		CacheHits:       atomic.LoadInt64(&d.metrics.cacheHits),
		CacheMisses:     atomic.LoadInt64(&d.metrics.cacheMisses),
	}
}

// Driver returns the database driver
func (d *Database) Driver() string {
	return d.driver
}

// Cleanup performs data cleanup
func (d *Database) Cleanup(ctx context.Context, before time.Time) error {
	// Batch deletion to avoid long transactions
	batchSize := d.opts.MaxBatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	for {
		query := "DELETE FROM metrics WHERE timestamp < ? LIMIT ?"
		if d.driver == "postgres" {
			query = ConvertPlaceholders(query)
		}
		result, err := d.ExecContext(ctx,
			query,
			before, batchSize)
		if err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}

		if affected < int64(batchSize) {
			break
		}

		// Avoid long-running transactions
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// RunPruning starts the data pruning task
func (d *Database) RunPruning(ctx context.Context) error {
	if d.pruneCtx != nil {
		return fmt.Errorf("pruning is already running")
	}

	d.pruneCtx, d.pruneCancel = context.WithCancel(ctx)
	go d.pruneLoop()
	return nil
}

// StopPruning stops the data pruning task
func (d *Database) StopPruning() error {
	if d.pruneCancel != nil {
		d.pruneCancel()
		d.pruneCancel = nil
		d.pruneCtx = nil
	}
	return nil
}

// Unwrap returns the underlying database connection
func (d *Database) Unwrap() *sql.DB {
	return d.db
}

// recordMetrics safely records operation metrics
func (d *Database) recordMetrics(start time.Time, err error) {
	duration := time.Since(start)

	atomic.AddInt64(&d.metrics.queryCount, 1)
	atomic.AddInt64(&d.metrics.queryTime, int64(duration))

	if err != nil {
		atomic.AddInt64(&d.metrics.queryErrors, 1)
		d.metrics.lastError = err
		d.metrics.lastErrorTime = time.Now()
	}

	if duration > d.opts.SlowQueryThreshold {
		atomic.AddInt64(&d.metrics.slowQueries, 1)
		d.logger.Warn("Slow query detected",
			zap.Duration("duration", duration))
	}
}

// pruneLoop handles periodic data pruning
func (d *Database) pruneLoop() {
	ticker := time.NewTicker(d.opts.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.pruneCtx.Done():
			return
		case <-ticker.C:
			pruneBefore := time.Now().Add(-d.opts.RetentionPeriod)
			if err := d.Cleanup(context.Background(), pruneBefore); err != nil {
				d.logger.Error("Failed to prune old data",
					zap.Error(err),
					zap.Time("before", pruneBefore))
			}
		}
	}
}

// healthCheck performs periodic health checks
func (d *Database) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.pruneCtx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := d.db.PingContext(ctx); err != nil {
				d.logger.Error("Database health check failed",
					zap.Error(err),
					zap.String("driver", d.driver))
				// Add retry logic
			}
			cancel()
		}
	}
}
