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

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// MySQLDatabase implements database interface for MySQL
type MySQLDatabase struct {
	*BaseDatabase
}

// NewMySQLDatabase creates a new MySQL database instance
func NewMySQLDatabase(dsn string, opts Options, logger *zap.Logger) (*MySQLDatabase, error) {
	// Configure MySQL specific settings
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %w", err)
	}

	cfg.ParseTime = true
	cfg.Timeout = opts.QueryTimeout
	cfg.ReadTimeout = opts.QueryTimeout
	cfg.WriteTimeout = opts.QueryTimeout

	base, err := NewBaseDatabase("mysql", cfg.FormatDSN(), opts, logger)
	if err != nil {
		return nil, err
	}

	database := &MySQLDatabase{
		BaseDatabase: base,
	}

	if err := database.initSchema(); err != nil {
		_ = base.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return database, nil
}

// initSchema creates MySQL tables
func (s *MySQLDatabase) initSchema() error {
	queries := []string{
		// metrics
		`CREATE TABLE IF NOT EXISTS metrics (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			agent_id VARCHAR(64) NOT NULL,
			timestamp DATETIME NOT NULL,
			collected_at DATETIME NOT NULL,
			reported_at DATETIME NOT NULL,
			data JSON NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_metrics_agent_time (agent_id, timestamp)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		// agents
		`CREATE TABLE IF NOT EXISTS agents (
			id VARCHAR(64) PRIMARY KEY,
			hostname VARCHAR(255) NOT NULL,
			version VARCHAR(32) NOT NULL,
			status VARCHAR(16) NOT NULL,
			last_seen DATETIME NOT NULL,
			registered_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			INDEX idx_agents_status (status),
			INDEX idx_agents_last_seen (last_seen)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		// ip changes
		`
    CREATE TABLE IF NOT EXISTS ip_changes (
        id BIGINT AUTO_INCREMENT PRIMARY KEY,
        agent_id VARCHAR(64) NOT NULL,
        interface_name VARCHAR(64),
        version VARCHAR(10) NOT NULL,
        is_external BOOLEAN NOT NULL,
        old_addrs JSON,
        new_addrs JSON,
        action VARCHAR(20) NOT NULL,
        reason VARCHAR(50) NOT NULL,
        timestamp DATETIME NOT NULL,
        created_at DATETIME NOT NULL,
        INDEX idx_agent_time (agent_id, timestamp),
        INDEX idx_interface (interface_name),
        INDEX idx_created_at (created_at)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
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
func (s *MySQLDatabase) StartPruning(ctx context.Context) error {
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
func (s *MySQLDatabase) StopPruning() error {
	if s.pruneStop != nil {
		close(s.pruneStop)
	}
	return nil
}

// SaveMetrics stores metrics data
func (s *MySQLDatabase) SaveMetrics(ctx context.Context, data *types.MetricsData) error {
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

// GetMetrics retrieves metrics
func (s *MySQLDatabase) GetMetrics(ctx context.Context, query *MetricsQuery, opts QueryOptions) ([]*types.MetricsData, error) {
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

// GetLatestMetrics retrieves the latest metrics
func (s *MySQLDatabase) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	query := `
        SELECT data FROM metrics
        WHERE agent_id = ?
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
func (s *MySQLDatabase) SaveIPChange(ctx context.Context, agentID string, change *types.IPChange) error {
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
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
func (s *MySQLDatabase) GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error) {
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
func (s *MySQLDatabase) Stats() Stats {
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

// Cleanup deletes old metrics
func (s *MySQLDatabase) Cleanup(ctx context.Context, before time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	// Delete old metrics in batches to avoid long locks
	batchSize := 10000
	for {
		query := "DELETE FROM metrics WHERE timestamp < ? LIMIT ?"
		result, err := tx.ExecContext(ctx, query, before, batchSize)
		if err != nil {
			return fmt.Errorf("failed to cleanup metrics: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}

		if affected < int64(batchSize) {
			break
		}

		// Commit batch and start new transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin new transaction: %w", err)
		}
	}

	return tx.Commit()
}
