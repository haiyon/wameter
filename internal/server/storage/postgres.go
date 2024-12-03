package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/haiyon/wameter/internal/types"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresStorage implements Storage interface for PostgreSQL
type PostgresStorage struct {
	*BaseStorage
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(dsn string, opts Options, logger *zap.Logger) (*PostgresStorage, error) {
	base, err := NewBaseStorage("postgres", dsn, opts, logger)
	if err != nil {
		return nil, err
	}

	storage := &PostgresStorage{
		BaseStorage: base,
	}

	if err := storage.initSchema(); err != nil {
		base.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return storage, nil
}

// initSchema creates PostgreSQL tables
func (s *PostgresStorage) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS metrics (
			id BIGSERIAL PRIMARY KEY,
			agent_id VARCHAR(64) NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			collected_at TIMESTAMP NOT NULL,
			reported_at TIMESTAMP NOT NULL,
			data JSONB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_agent_time
		ON metrics(agent_id, timestamp)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id VARCHAR(64) PRIMARY KEY,
			hostname VARCHAR(255) NOT NULL,
			version VARCHAR(32) NOT NULL,
			status VARCHAR(16) NOT NULL,
			last_seen TIMESTAMP NOT NULL,
			registered_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen)`,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, q := range queries {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("failed to exec query %s: %w", q, err)
		}
	}

	return tx.Commit()
}

// StartPruning starts the background pruning routine
func (s *PostgresStorage) StartPruning(ctx context.Context) error {
	if !s.opts.EnablePruning {
		return nil
	}

	s.pruneStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(s.opts.PruneInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.pruneStop:
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-s.opts.MetricsRetention)
				if err := s.Cleanup(ctx, cutoff); err != nil {
					s.logger.Error("Failed to prune old metrics", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

// StopPruning stops the pruning routine
func (s *PostgresStorage) StopPruning() error {
	if s.pruneStop != nil {
		close(s.pruneStop)
	}
	return nil
}

// SaveMetrics stores metrics
func (s *PostgresStorage) SaveMetrics(ctx context.Context, data *types.MetricsData) error {
	query := `
		INSERT INTO metrics (agent_id, timestamp, collected_at, reported_at, data)
		VALUES ($1, $2, $3, $4, $5)`

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	_, err = s.ExecContext(ctx, query,
		data.AgentID,
		data.Timestamp,
		data.CollectedAt,
		data.ReportedAt,
		jsonData)

	if err != nil {
		return fmt.Errorf("failed to save metrics: %w", err)
	}

	return nil
}

// GetMetrics retrieves metrics
func (s *PostgresStorage) GetMetrics(ctx context.Context, query *MetricsQuery, opts QueryOptions) ([]*types.MetricsData, error) {
	qb := &QueryBuilder{}
	qb.Reset()

	baseQuery := `
		SELECT data FROM metrics
		WHERE timestamp BETWEEN $1 AND $2`

	qb.args = append(qb.args, query.StartTime, query.EndTime)

	if len(query.AgentIDs) > 0 {
		qb.Where("agent_id = ANY($?)", pq.Array(query.AgentIDs))
	}

	if len(query.MetricTypes) > 0 {
		qb.Where("data->>'type' = ANY($?)", pq.Array(query.MetricTypes))
	}

	if query.OrderBy != "" {
		qb.OrderBy(query.OrderBy, query.Order)
	}

	if query.Limit > 0 {
		qb.Limit(query.Limit)
	}

	queryCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	rows, err := s.QueryContext(queryCtx, baseQuery+qb.String(), qb.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var results []*types.MetricsData
	for rows.Next() {
		var data types.MetricsData
		if err := scanMetrics(rows, &data); err != nil {
			return nil, err
		}
		results = append(results, &data)
	}

	return results, rows.Err()
}

// GetLatestMetrics retrieves latest metrics
func (s *PostgresStorage) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	query := `
        SELECT data FROM metrics
        WHERE agent_id = $1
        ORDER BY timestamp DESC
        LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, agentID)

	var data types.MetricsData
	var jsonData []byte
	if err := row.Scan(&jsonData); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, types.ErrAgentNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetAgent retrieves an agent
func (s *PostgresStorage) GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error) {
	query := `
        SELECT id, hostname, version, status, last_seen, registered_at, updated_at
        FROM agents WHERE id = $1`

	row := s.db.QueryRowContext(ctx, query, agentID)

	agent := &types.AgentInfo{}
	err := row.Scan(
		&agent.ID,
		&agent.Hostname,
		&agent.Version,
		&agent.Status,
		&agent.LastSeen,
		&agent.RegisteredAt,
		&agent.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, types.ErrAgentNotFound
	}
	return agent, err
}

// Stats returns database statistics
func (s *PostgresStorage) Stats() Stats {
	dbStats := s.db.Stats()
	return Stats{
		OpenConnections:   dbStats.OpenConnections,
		InUse:             dbStats.InUse,
		Idle:              dbStats.Idle,
		WaitCount:         dbStats.WaitCount,
		WaitDuration:      dbStats.WaitDuration,
		MaxIdleClosed:     dbStats.MaxIdleClosed,
		MaxLifetimeClosed: dbStats.MaxLifetimeClosed,
		QueryCount:        atomic.LoadInt64(&s.metrics.QueryCount),
		QueryErrors:       atomic.LoadInt64(&s.metrics.QueryErrors),
		SlowQueries:       atomic.LoadInt64(&s.metrics.SlowQueryCount),
	}
}

// Cleanup deletes old metrics data
func (s *PostgresStorage) Cleanup(ctx context.Context, before time.Time) error {
	// PostgreSQL can handle large deletes efficiently
	query := "DELETE FROM metrics WHERE timestamp < $1"
	result, err := s.db.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup metrics: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	s.logger.Info("Cleanup completed",
		zap.Time("before", before),
		zap.Int64("deleted", affected))

	return nil
}
