package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"wameter/internal/types"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// SQLiteDatabase implements Database interface for SQLite
type SQLiteDatabase struct {
	*BaseDatabase
}

// NewSQLiteDatabase creates a new SQLite database instance
func NewSQLiteDatabase(dsn string, opts Options, logger *zap.Logger) (*SQLiteDatabase, error) {
	base, err := NewBaseDatabase("sqlite3", dsn, opts, logger)
	if err != nil {
		return nil, err
	}

	database := &SQLiteDatabase{
		BaseDatabase: base,
	}

	if err := database.initSchema(); err != nil {
		base.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return database, nil
}

// initSchema creates SQLite tables
func (s *SQLiteDatabase) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS agents (
            id TEXT PRIMARY KEY,
            hostname TEXT NOT NULL,
            version TEXT NOT NULL,
            status TEXT NOT NULL,
            last_seen DATETIME NOT NULL,
            registered_at DATETIME NOT NULL,
            updated_at DATETIME NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS metrics (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            agent_id TEXT NOT NULL,
            timestamp DATETIME NOT NULL,
            collected_at DATETIME NOT NULL,
            reported_at DATETIME NOT NULL,
            data JSON NOT NULL,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (agent_id) REFERENCES agents(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_agent_time
         ON metrics(agent_id, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_status
         ON agents(status)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_last_seen
         ON agents(last_seen)`,
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
func (s *SQLiteDatabase) StartPruning(ctx context.Context) error {
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
func (s *SQLiteDatabase) StopPruning() error {
	if s.pruneStop != nil {
		close(s.pruneStop)
	}
	return nil
}

// SaveMetrics stores metrics data
func (s *SQLiteDatabase) SaveMetrics(ctx context.Context, data *types.MetricsData) error {
	query := `
		INSERT INTO metrics (agent_id, timestamp, collected_at, reported_at, data)
		VALUES (?, ?, ?, ?, ?)`

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

// GetMetrics retrieves metrics based on query
func (s *SQLiteDatabase) GetMetrics(ctx context.Context, query *MetricsQuery, opts QueryOptions) ([]*types.MetricsData, error) {
	qb := &QueryBuilder{}
	qb.Reset()

	baseQuery := `
		SELECT data FROM metrics
		WHERE timestamp BETWEEN ? AND ?`

	qb.args = append(qb.args, query.StartTime, query.EndTime)

	if len(query.AgentIDs) > 0 {
		qb.Where("agent_id IN (?)", query.AgentIDs)
	}

	if len(query.MetricTypes) > 0 {
		qb.Where("JSON_EXTRACT(data, '$.type') IN (?)", query.MetricTypes)
	}

	// Add sorting
	if query.OrderBy != "" {
		qb.OrderBy(query.OrderBy, query.Order)
	}

	// Add limit
	if query.Limit > 0 {
		qb.Limit(query.Limit)
	}

	// Execute query with timeout
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

// GetLatestMetrics retrieves the latest metrics
func (s *SQLiteDatabase) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	query := `
        SELECT data FROM metrics
        WHERE agent_id = ?
        ORDER BY timestamp DESC
        LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, agentID)

	var data types.MetricsData
	var jsonData []byte
	if err := row.Scan(&jsonData); err != nil {
		if err == sql.ErrNoRows {
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
func (s *SQLiteDatabase) GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error) {
	query := `
        SELECT id, hostname, version, status, last_seen, registered_at, updated_at
        FROM agents WHERE id = ?`

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
func (s *SQLiteDatabase) Stats() Stats {
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
func (s *SQLiteDatabase) Cleanup(ctx context.Context, before time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	// Delete old metrics
	query := "DELETE FROM metrics WHERE timestamp < ?"
	result, err := tx.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup metrics: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	// Log cleanup stats
	s.logger.Info("Cleanup completed",
		zap.Time("before", before),
		zap.Int64("deleted", deleted))

	return tx.Commit()
}
