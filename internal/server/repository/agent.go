package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"wameter/internal/database"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// agentRepository represents agent repository implementation
type agentRepository struct {
	db     database.Interface
	logger *zap.Logger
}

// NewAgentRepository creates new agent repository
func NewAgentRepository(db database.Interface, logger *zap.Logger) AgentRepository {
	return &agentRepository{
		db:     db,
		logger: logger,
	}
}

// Save saves or updates an agent
func (r *agentRepository) Save(ctx context.Context, agent *types.AgentInfo) error {
	query := `INSERT INTO agents (
                id, hostname, version, status,
                last_seen, registered_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?)`

	if r.db.Driver() == "postgres" {
		query += `ON CONFLICT (id) DO UPDATE SET
                hostname = EXCLUDED.hostname,
                version = EXCLUDED.version,
                status = EXCLUDED.status,
                last_seen = EXCLUDED.last_seen,
                updated_at = EXCLUDED.updated_at`
		// Convert placeholders for postgres
		query = database.ConvertPlaceholders(query)
	} else if r.db.Driver() == "mysql" {
		query += `ON DUPLICATE KEY UPDATE
                hostname = VALUES(hostname),
                version = VALUES(version),
                status = VALUES(status),
                last_seen = VALUES(last_seen),
                updated_at = VALUES(updated_at)`
	} else if r.db.Driver() == "sqlite" {
		query = `INSERT INTO agents (
                id, hostname, version, status,
                last_seen, registered_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?)`
	}

	result, err := r.db.ExecContext(ctx, query,
		agent.ID, agent.Hostname, agent.Version,
		agent.Status, agent.LastSeen, agent.RegisteredAt,
		agent.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no rows affected")
	}

	return nil
}

// FindByID returns agent by ID
func (r *agentRepository) FindByID(ctx context.Context, id string) (*types.AgentInfo, error) {
	query := `
        SELECT id, hostname, version, status,
               last_seen, registered_at, updated_at
        FROM agents
        WHERE id = ?`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	var agent types.AgentInfo
	err := r.db.QueryRowContext(ctx, query, id).Scan(
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
	if err != nil {
		return nil, fmt.Errorf("failed to query agent: %w", err)
	}

	return &agent, nil
}

// UpdateAgent updates an existing agent
func (r *agentRepository) UpdateAgent(ctx context.Context, agent *types.AgentInfo) error {
	qb := database.NewQueryBuilder(r.db.Driver())
	qb.Raw(
		"UPDATE agents SET hostname = ?, version = ?, status = ?, last_seen = ?, updated_at = ? WHERE id = ?",
		agent.Hostname,
		agent.Version,
		agent.Status,
		agent.LastSeen,
		time.Now(),
		agent.ID,
	)

	result, err := r.db.ExecContext(ctx, qb.SQL(), qb.Args()...)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return types.ErrAgentNotFound
	}

	return nil
}

// UpdateStatus updates agent status
func (r *agentRepository) UpdateStatus(ctx context.Context, id string, status types.AgentStatus) error {
	query := `
        UPDATE agents
        SET status = ?, last_seen = ?, updated_at = ?
        WHERE id = ?`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query,
		status, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return types.ErrAgentNotFound
	}

	return nil
}

// List returns all agents
func (r *agentRepository) List(ctx context.Context) ([]*types.AgentInfo, error) {
	qb := database.NewQueryBuilder(r.db.Driver())
	qb.Select("id, hostname, version, status, last_seen, registered_at, updated_at").
		From("agents").
		OrderBy("hostname")

	rows, err := r.db.QueryContext(ctx, qb.SQL(), qb.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

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
			&agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}
		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, nil
}

// ListWithPagination returns agents with pagination
func (r *agentRepository) ListWithPagination(ctx context.Context, limit, offset int) ([]*types.AgentInfo, error) {
	qb := database.NewQueryBuilder(r.db.Driver())

	qb.Select("id, hostname, version, status, last_seen, registered_at, updated_at").
		From("agents").
		OrderBy("hostname").
		Limit(limit).
		Offset(offset)

	rows, err := r.db.QueryContext(ctx, qb.SQL(), qb.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var agents []*types.AgentInfo
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled while scanning agents: %w", err)
		}

		agent := &types.AgentInfo{}
		err := rows.Scan(
			&agent.ID,
			&agent.Hostname,
			&agent.Version,
			&agent.Status,
			&agent.LastSeen,
			&agent.RegisteredAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}
		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, nil
}

// Delete deletes an agent and all associated data
func (r *agentRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Delete associated metrics first
		if err := r.deleteAgentMetrics(ctx, tx, id); err != nil {
			return err
		}

		// Delete associated IP changes
		if err := r.deleteAgentIPChanges(ctx, tx, id); err != nil {
			return err
		}

		// Delete the agent
		query := "DELETE FROM agents WHERE id = ?"
		if r.db.Driver() == "postgres" {
			query = database.ConvertPlaceholders(query)
		}

		result, err := tx.ExecContext(ctx, query, id)
		if err != nil {
			return fmt.Errorf("failed to delete agent: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}

		if affected == 0 {
			return types.ErrAgentNotFound
		}

		return nil
	})
}

// GetAgentMetrics retrieves agent metrics
func (r *agentRepository) GetAgentMetrics(ctx context.Context, id string) (*types.AgentMetrics, error) {
	metrics := &types.AgentMetrics{}

	// Get basic metrics
	qb := database.NewQueryBuilder(r.db.Driver())
	qb.Select(
		"COUNT(*) as total_collections",
		"SUM(CASE WHEN a.status = 'error' THEN 1 ELSE 0 END) as failed_collections",
		"MAX(CASE WHEN a.status = 'offline' THEN m.timestamp END) as last_downtime",
	).
		From("metrics m").
		Join("INNER", "agents a", "m.agent_id = a.id").
		Where("m.agent_id = ?", id)

	err := r.db.QueryRowContext(ctx, qb.SQL(), qb.Args()...).Scan(
		&metrics.TotalCollections,
		&metrics.FailedCollections,
		&metrics.LastDowntime,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get agent metrics: %w", err)
	}

	// Calculate uptime percentage
	if metrics.TotalCollections > 0 {
		metrics.UptimePercent = float64(metrics.TotalCollections-metrics.FailedCollections) / float64(metrics.TotalCollections) * 100
	}

	// Get network statistics
	if err := r.getAgentNetworkStats(ctx, id, metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

// getAgentNetworkStats retrieves network statistics for an agent
func (r *agentRepository) getAgentNetworkStats(ctx context.Context, id string, metrics *types.AgentMetrics) error {
	qb := database.NewQueryBuilder(r.db.Driver())
	qb.Select(
		"COUNT(DISTINCT data->'metrics'->'network'->'interfaces') as interface_count",
		"SUM(CAST(data->'metrics'->'network'->'total_bandwidth' AS BIGINT)) as total_bandwidth",
		"AVG(CAST(data->'metrics'->'network'->'error_rate' AS FLOAT)) as error_rate",
	).
		From("metrics").
		Where("agent_id = ?", id).
		Where("data->'metrics'->>'network' IS NOT NULL")

	err := r.db.QueryRowContext(ctx, qb.SQL(), qb.Args()...).Scan(
		&metrics.NetworkStats.InterfaceCount,
		&metrics.NetworkStats.TotalBandwidth,
		&metrics.NetworkStats.ErrorRate,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to get agent network stats: %w", err)
	}

	return nil
}

// deleteAgentMetrics deletes all metrics for an agent
func (r *agentRepository) deleteAgentMetrics(ctx context.Context, tx *sql.Tx, id string) error {
	query := "DELETE FROM metrics WHERE agent_id = ?"
	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	_, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent metrics: %w", err)
	}

	return nil
}

// deleteAgentIPChanges deletes all IP changes for an agent
func (r *agentRepository) deleteAgentIPChanges(ctx context.Context, tx *sql.Tx, id string) error {
	query := "DELETE FROM ip_changes WHERE agent_id = ?"
	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	_, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent ip changes: %w", err)
	}

	return nil
}
