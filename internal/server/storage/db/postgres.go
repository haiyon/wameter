package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"wameter/internal/types"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresDatabase implements database interface for PostgreSQL
type PostgresDatabase struct {
	*BaseDatabase
}

// NewPostgresDatabase creates a new PostgreSQL database instance
func NewPostgresDatabase(dsn string, opts Options, logger *zap.Logger) (*PostgresDatabase, error) {
	base, err := NewBaseDatabase("postgres", dsn, opts, logger)
	if err != nil {
		return nil, err
	}

	database := &PostgresDatabase{
		BaseDatabase: base,
	}

	if err := database.initSchema(); err != nil {
		_ = base.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return database, nil
}

// initSchema creates PostgreSQL tables
func (s *PostgresDatabase) initSchema() error {
	queries := []string{
		// metrics
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
		// agents
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
		// ip changes
		`CREATE TABLE IF NOT EXISTS ip_changes (
        id BIGSERIAL PRIMARY KEY,
        agent_id VARCHAR(64) NOT NULL,
        interface_name VARCHAR(64),
        version VARCHAR(10) NOT NULL,
        is_external BOOLEAN NOT NULL,
        old_addrs JSONB,
        new_addrs JSONB,
        action VARCHAR(20) NOT NULL,
        reason VARCHAR(50) NOT NULL,
        timestamp TIMESTAMP NOT NULL,
        created_at TIMESTAMP NOT NULL
    );
    CREATE INDEX IF NOT EXISTS idx_ip_changes_agent_time ON ip_changes(agent_id, timestamp);
    CREATE INDEX IF NOT EXISTS idx_ip_changes_interface ON ip_changes(interface_name);
    CREATE INDEX IF NOT EXISTS idx_ip_changes_created_at ON ip_changes(created_at)`,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	for _, q := range queries {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("failed to exec query %s: %w", q, err)
		}
	}

	return tx.Commit()
}

// StartPruning starts the background pruning routine
func (s *PostgresDatabase) StartPruning(ctx context.Context) error {
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
func (s *PostgresDatabase) StopPruning() error {
	if s.pruneStop != nil {
		close(s.pruneStop)
	}
	return nil
}

// SaveMetrics stores metrics
func (s *PostgresDatabase) SaveMetrics(ctx context.Context, data *types.MetricsData) error {
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
func (s *PostgresDatabase) GetMetrics(ctx context.Context, query *MetricsQuery, opts QueryOptions) ([]*types.MetricsData, error) {
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
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

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
func (s *PostgresDatabase) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
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

// SaveIPChange stores an IP change
func (s *PostgresDatabase) SaveIPChange(ctx context.Context, agentID string, change *types.IPChange) error {
	oldAddrs, err := json.Marshal(change.OldAddrs)
	if err != nil {
		return fmt.Errorf("failed to marshal old addresses: %w", err)
	}

	newAddrs, err := json.Marshal(change.NewAddrs)
	if err != nil {
		return fmt.Errorf("failed to marshal new addresses: %w", err)
	}

	query := `
        INSERT INTO ip_changes (
            agent_id, interface_name, version, is_external,
            old_addrs, new_addrs, action, reason, timestamp, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err = s.ExecContext(ctx, query,
		agentID, change.InterfaceName, change.Version, change.IsExternal,
		oldAddrs, newAddrs, change.Action, change.Reason,
		change.Timestamp, time.Now())

	if err != nil {
		return fmt.Errorf("failed to save IP change: %w", err)
	}

	return nil
}

// GetAgent retrieves an agent
func (s *PostgresDatabase) GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error) {
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
func (s *PostgresDatabase) Stats() Stats {
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
func (s *PostgresDatabase) Cleanup(ctx context.Context, before time.Time) error {
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
