package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
	"wameter/internal/utils"

	"wameter/internal/types"

	"go.uber.org/zap"
)

// Options defines database options
type Options struct {
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	ConnMaxIdleTime  time.Duration
	QueryTimeout     time.Duration
	EnablePruning    bool
	MetricsRetention time.Duration
	PruneInterval    time.Duration
}

// BaseDatabase is the base implementation of the Database interface
type BaseDatabase struct {
	driver    string
	db        *sql.DB
	opts      Options
	logger    *zap.Logger
	metrics   *Metrics
	pruneStop chan struct{}
}

// NewBaseDatabase creates new BaseDatabase
func NewBaseDatabase(driver, dsn string, opts Options, logger *zap.Logger) (*BaseDatabase, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool
	db.SetMaxOpenConns(opts.MaxOpenConns)
	db.SetMaxIdleConns(opts.MaxIdleConns)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	db.SetConnMaxIdleTime(opts.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		err := db.Close()
		if err != nil {
			logger.Error("failed to close database", zap.Error(err))
			return nil, err
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &BaseDatabase{
		driver:  driver,
		db:      db,
		opts:    opts,
		logger:  logger,
		metrics: &Metrics{},
	}, nil
}

// RegisterAgent registers a new agent or updates existing one
func (s *BaseDatabase) RegisterAgent(ctx context.Context, agent *types.AgentInfo) error {
	return s.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Check if the ID already exists
		existsQuery := "SELECT COUNT(*) FROM agents WHERE id = ?"
		if s.driver == "postgres" {
			existsQuery = utils.ConvertPlaceholders(existsQuery)
		}
		var count int
		err := tx.QueryRowContext(ctx, existsQuery, agent.ID).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check agent existence: %w", err)
		}

		if count > 0 {
			// Exists, execute update
			updateQuery := `UPDATE agents SET hostname = ?, version = ?, status = ?, last_seen = ?, updated_at = ? WHERE id = ?`
			if s.driver == "postgres" {
				updateQuery = utils.ConvertPlaceholders(updateQuery)
			}
			_, err = tx.ExecContext(ctx, updateQuery,
				agent.Hostname,
				agent.Version,
				agent.Status,
				agent.LastSeen,
				agent.UpdatedAt,
				agent.ID)
		} else {
			// Doesn't exist, execute insert
			insertQuery := ` INSERT INTO agents (id, hostname, version, status, last_seen, registered_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
			if s.driver == "postgres" {
				insertQuery = utils.ConvertPlaceholders(insertQuery)
			}
			_, err = tx.ExecContext(ctx, insertQuery,
				agent.ID,
				agent.Hostname,
				agent.Version,
				agent.Status,
				agent.LastSeen,
				agent.RegisteredAt,
				agent.UpdatedAt)
		}

		return err
	})
}

// UpdateAgentStatus updates agent status
func (s *BaseDatabase) UpdateAgentStatus(ctx context.Context, agentID string, status types.AgentStatus) error {
	query := `
        UPDATE agents
        SET status = ?, last_seen = ?, updated_at = ?
        WHERE id = ?`

	now := time.Now()
	result, err := s.ExecContext(ctx, query, status, now, now, agentID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return types.ErrAgentNotFound
	}

	return nil
}

// GetAgents retrieves all agents
func (s *BaseDatabase) GetAgents(ctx context.Context) ([]*types.AgentInfo, error) {
	query := `
        SELECT id, hostname, version, status, last_seen, registered_at, updated_at
        FROM agents
        ORDER BY hostname`

	// Execute query with timeout
	rows, err := s.QueryContext(ctx, query)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var agents []*types.AgentInfo
	for rows.Next() {
		// Check context cancellation during iteration
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		agent := &types.AgentInfo{}
		err := rows.Scan(
			&agent.ID,
			&agent.Hostname,
			&agent.Version,
			&agent.Status,
			&agent.LastSeen,
			&agent.RegisteredAt,
			&agent.UpdatedAt)

		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return agents, nil
}

// BatchSaveMetrics saves multiple metrics in a single transaction
func (s *BaseDatabase) BatchSaveMetrics(ctx context.Context, metrics []*types.MetricsData) error {
	if len(metrics) == 0 {
		return nil
	}

	return s.WithTransaction(ctx, func(tx *sql.Tx) error {
		query := `
            INSERT INTO metrics (agent_id, timestamp, collected_at, reported_at, data)
            VALUES (?, ?, ?, ?, ?)`

		if s.driver == "postgres" {
			query = utils.ConvertPlaceholders(query)
		}

		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, m := range metrics {
			jsonData, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("marshal metrics data: %w", err)
			}

			_, err = stmt.ExecContext(ctx,
				m.AgentID,
				m.Timestamp,
				m.CollectedAt,
				m.ReportedAt,
				jsonData)
			if err != nil {
				return fmt.Errorf("exec statement: %w", err)
			}
		}

		return nil
	})
}

// TxFn represents a transaction function
type TxFn func(*sql.Tx) error

// WithTransaction executes operations in a transaction
func (s *BaseDatabase) WithTransaction(ctx context.Context, fn TxFn) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error("rollback failed during panic",
					zap.Error(rbErr),
					zap.Any("panic", p))
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.Error("rollback failed",
				zap.Error(rbErr),
				zap.Error(err))
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

// ExecContext executes a query
func (s *BaseDatabase) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	// Timeout
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()

	if s.driver == "postgres" {
		query = utils.ConvertPlaceholders(query)
	}

	start := time.Now()
	result, err := s.db.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	// Update metrics
	atomic.AddInt64(&s.metrics.QueryCount, 1)
	if err != nil {
		atomic.AddInt64(&s.metrics.QueryErrors, 1)
		s.metrics.LastError = err
		s.metrics.LastErrorTime = time.Now()
	}

	// Log slow queries
	if duration > time.Second {
		atomic.AddInt64(&s.metrics.SlowQueryCount, 1)
		s.logger.Warn("slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}

	return result, err
}

// QueryContext executes a query
func (s *BaseDatabase) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	// Timeout
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()

	if s.driver == "postgres" {
		query = utils.ConvertPlaceholders(query)
	}

	start := time.Now()
	rows, err := s.db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	// Update metrics
	atomic.AddInt64(&s.metrics.QueryCount, 1)
	if err != nil {
		atomic.AddInt64(&s.metrics.QueryErrors, 1)
		s.metrics.LastError = err
		s.metrics.LastErrorTime = time.Now()
	}

	// Log slow queries
	if duration > time.Second {
		atomic.AddInt64(&s.metrics.SlowQueryCount, 1)
		s.logger.Warn("slow query detected",
			zap.String("query", query),
			zap.Duration("duration", duration))
	}

	return rows, err
}

// Close closes the database
func (s *BaseDatabase) Close() error {
	return s.db.Close()
}

// Ping pings the database
func (s *BaseDatabase) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Stats returns database statistics
func (s *BaseDatabase) Stats() *Stats {
	dbStats := s.db.Stats()
	return &Stats{
		OpenConnections:   dbStats.OpenConnections,
		InUse:             dbStats.InUse,
		Idle:              dbStats.Idle,
		WaitCount:         dbStats.WaitCount,
		WaitDuration:      dbStats.WaitDuration,
		MaxIdleClosed:     dbStats.MaxIdleClosed,
		MaxLifetimeClosed: dbStats.MaxLifetimeClosed,
	}
}

// scanMetrics scans a row into MetricsData
func scanMetrics(rows *sql.Rows, data *types.MetricsData) error {
	var jsonData []byte
	if err := rows.Scan(&jsonData); err != nil {
		return err
	}
	return json.Unmarshal(jsonData, data)
}

// GetMetrics returns metrics
func (s *BaseDatabase) GetMetrics(ctx context.Context, query *MetricsQuery, opts QueryOptions) ([]*types.MetricsData, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	queryCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	qb := &QueryBuilder{}
	qb.Reset()

	baseQuery := `
        SELECT data FROM metrics
        WHERE timestamp BETWEEN ? AND ?`

	qb.args = append(qb.args, query.StartTime, query.EndTime)

	if len(query.AgentIDs) > 0 {
		qb.Where("agent_id IN (?)", query.AgentIDs)
	}

	if query.OrderBy != "" {
		qb.OrderBy(query.OrderBy, query.Order)
	}

	if query.Limit > 0 {
		qb.Limit(query.Limit)
	}

	rows, err := s.QueryContext(queryCtx, baseQuery+qb.String(), qb.Args()...)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	var results []*types.MetricsData
	for rows.Next() {
		if err := queryCtx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var data types.MetricsData
		if err := scanMetrics(rows, &data); err != nil {
			return nil, fmt.Errorf("scan metrics: %w", err)
		}
		results = append(results, &data)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return results, nil
}
