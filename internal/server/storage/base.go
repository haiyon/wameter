package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/haiyon/wameter/internal/types"
	"go.uber.org/zap"
)

// Options defines storage options
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

// BaseStorage is the base implementation of the Storage interface
type BaseStorage struct {
	db        *sql.DB
	opts      Options
	logger    *zap.Logger
	metrics   *Metrics
	pruneStop chan struct{}
}

// NewBaseStorage creates new BaseStorage
func NewBaseStorage(driver, dsn string, opts Options, logger *zap.Logger) (*BaseStorage, error) {
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

	return &BaseStorage{
		db:      db,
		opts:    opts,
		logger:  logger,
		metrics: &Metrics{},
	}, nil
}

// RegisterAgent registers a new agent or updates existing one
func (s *BaseStorage) RegisterAgent(ctx context.Context, agent *types.AgentInfo) error {
	query := `
        INSERT INTO agents (id, hostname, version, status, last_seen, registered_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (id) DO UPDATE SET
            hostname = EXCLUDED.hostname,
            version = EXCLUDED.version,
            status = EXCLUDED.status,
            last_seen = EXCLUDED.last_seen,
            updated_at = EXCLUDED.updated_at`

	_, err := s.ExecContext(ctx, query,
		agent.ID,
		agent.Hostname,
		agent.Version,
		agent.Status,
		agent.LastSeen,
		agent.RegisteredAt,
		agent.UpdatedAt)

	return err
}

// UpdateAgentStatus updates agent status
func (s *BaseStorage) UpdateAgentStatus(ctx context.Context, agentID string, status types.AgentStatus) error {
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
func (s *BaseStorage) GetAgents(ctx context.Context) ([]*types.AgentInfo, error) {
	query := `
        SELECT id, hostname, version, status, last_seen, registered_at, updated_at
        FROM agents`

	rows, err := s.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*types.AgentInfo
	for rows.Next() {
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
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// BatchSaveMetrics saves multiple metrics in a single transaction
func (s *BaseStorage) BatchSaveMetrics(ctx context.Context, metrics []*types.MetricsData) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
        INSERT INTO metrics (agent_id, timestamp, collected_at, reported_at, data)
        VALUES (?, ?, ?, ?, ?)`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range metrics {
		jsonData, err := json.Marshal(m)
		if err != nil {
			return err
		}

		_, err = stmt.ExecContext(ctx,
			m.AgentID,
			m.Timestamp,
			m.CollectedAt,
			m.ReportedAt,
			jsonData)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// TxFn represents a transaction function
type TxFn func(*sql.Tx) error

// WithTransaction executes operations in a transaction
func (s *BaseStorage) WithTransaction(ctx context.Context, fn TxFn) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// ExecContext executes a query
func (s *BaseStorage) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	// Timeout
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()

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
func (s *BaseStorage) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	// Timeout
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()

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
func (s *BaseStorage) Close() error {
	return s.db.Close()
}

// Ping pings the database
func (s *BaseStorage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Stats returns database statistics
func (s *BaseStorage) Stats() *Stats {
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
