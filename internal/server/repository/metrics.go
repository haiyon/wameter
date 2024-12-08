package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"wameter/internal/database"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// MetricsRepository represents metrics repository implementation
type metricsRepository struct {
	db     database.Interface
	logger *zap.Logger
}

// NewMetricsRepository creates new metrics repository
func NewMetricsRepository(db database.Interface, logger *zap.Logger) MetricsRepository {
	return &metricsRepository{
		db:     db,
		logger: logger,
	}
}

// Save saves metrics
func (r *metricsRepository) Save(ctx context.Context, data *types.MetricsData) error {
	query := `
        INSERT INTO metrics (
            agent_id, timestamp, collected_at,
            reported_at, data, created_at
        ) VALUES (?, ?, ?, ?, ?, ?)`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics data: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		data.AgentID,
		data.Timestamp,
		data.CollectedAt,
		data.ReportedAt,
		jsonData,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save metrics: %w", err)
	}

	return nil
}

// BatchSave saves multiple metrics
func (r *metricsRepository) BatchSave(ctx context.Context, metrics []*types.MetricsData) error {
	return r.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		query := `
            INSERT INTO metrics (
                agent_id, timestamp, collected_at,
                reported_at, data, created_at
            ) VALUES (?, ?, ?, ?, ?, ?)`

		if r.db.Driver() == "postgres" {
			query = database.ConvertPlaceholders(query)
		}

		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}

		defer func(stmt *sql.Stmt) {
			_ = stmt.Close()
		}(stmt)

		for _, m := range metrics {
			jsonData, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("failed to marshal metrics: %w", err)
			}

			_, err = stmt.ExecContext(ctx,
				m.AgentID,
				m.Timestamp,
				m.CollectedAt,
				m.ReportedAt,
				jsonData,
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to save metrics: %w", err)
			}
		}

		return nil
	})
}

// Query returns metrics based on query parameters
func (r *metricsRepository) Query(ctx context.Context, params QueryParams) ([]*types.MetricsData, error) {
	qb := database.NewQueryBuilder(r.db.Driver())

	qb.Select("data")
	qb.From("metrics")
	qb.Where("timestamp BETWEEN ? AND ?", params.StartTime, params.EndTime)

	if len(params.AgentIDs) > 0 {
		placeholders := strings.Repeat("?,", len(params.AgentIDs))
		placeholders = placeholders[:len(placeholders)-1]
		qb.Where(fmt.Sprintf("agent_id IN (%s)", placeholders), interfaceSlice(params.AgentIDs)...)
	}

	if params.OrderBy != "" {
		direction := "ASC"
		if params.Order != "" {
			direction = params.Order
		}
		qb.OrderBy(fmt.Sprintf("%s %s", params.OrderBy, direction))
	}

	if params.Limit > 0 {
		qb.Limit(params.Limit)
	}

	if params.Offset > 0 {
		qb.Offset(params.Offset)
	}

	rows, err := r.db.QueryContext(ctx, qb.SQL(), qb.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var results []*types.MetricsData
	for rows.Next() {
		var jsonData []byte
		if err := rows.Scan(&jsonData); err != nil {
			return nil, fmt.Errorf("failed to scan metrics: %w", err)
		}

		var data types.MetricsData
		if err := json.Unmarshal(jsonData, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}

		results = append(results, &data)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics: %w", err)
	}

	return results, nil
}

// interfaceSlice converts []string to []any
func interfaceSlice(slice []string) []any {
	is := make([]any, len(slice))
	for i, v := range slice {
		is[i] = v
	}
	return is
}

// GetLatest returns the latest metrics for the given agent
func (r *metricsRepository) GetLatest(ctx context.Context, agentID string) (*types.MetricsData, error) {
	query := `
        SELECT data
        FROM metrics
        WHERE agent_id = ?
        ORDER BY timestamp DESC
        LIMIT 1`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	var jsonData []byte
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(&jsonData)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, types.ErrAgentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}

	var data types.MetricsData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return &data, nil
}

// DeleteBefore deletes metrics before the given time
func (r *metricsRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	query := "DELETE FROM metrics WHERE timestamp < ?"
	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to delete metrics: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	r.logger.Info("Deleted old metrics",
		zap.Int64("count", affected),
		zap.Time("before", before))

	return nil
}

// GetMetricsByTimeRange retrieves metrics within a time range
func (r *metricsRepository) GetMetricsByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*types.MetricsData, error) {
	qb := database.NewQueryBuilder(r.db.Driver())
	qb.Select("data").
		From("metrics").
		Where("timestamp BETWEEN ? AND ?", startTime, endTime).
		OrderBy("timestamp DESC")

	rows, err := r.db.QueryContext(ctx, qb.SQL(), qb.Args()...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var results []*types.MetricsData
	for rows.Next() {
		var data types.MetricsData
		var jsonData []byte

		if err := rows.Scan(&jsonData); err != nil {
			return nil, fmt.Errorf("failed to scan metrics: %w", err)
		}

		if err := json.Unmarshal(jsonData, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}

		results = append(results, &data)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics: %w", err)
	}

	return results, nil
}

// GetMetricsSummary returns a summary of metrics for an agent
func (r *metricsRepository) GetMetricsSummary(ctx context.Context, agentID string) (*types.MetricsSummary, error) {
	query := `
        SELECT
            COUNT(*) as total_metrics,
            MIN(timestamp) as first_seen,
            MAX(timestamp) as last_seen
        FROM metrics
        WHERE agent_id = ?`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	summary := &types.MetricsSummary{}
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&summary.TotalMetrics,
		&summary.FirstSeen,
		&summary.LastSeen,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics summary: %w", err)
	}

	// Get network metrics summary
	if err := r.getNetworkMetricsSummary(ctx, agentID, summary); err != nil {
		r.logger.Error("Failed to get network metrics summary",
			zap.Error(err),
			zap.String("agent_id", agentID))
	}

	return summary, nil
}

// getNetworkMetricsSummary retrieves network-specific metrics summary
func (r *metricsRepository) getNetworkMetricsSummary(ctx context.Context, agentID string, summary *types.MetricsSummary) error {
	query := `
        SELECT
            SUM(
                CAST(data->'metrics'->'network'->>'total_traffic' AS BIGINT)
            ) as total_traffic,
            AVG(
                CAST(data->'metrics'->'network'->>'utilization' AS FLOAT)
            ) as avg_utilization,
            COUNT(DISTINCT data->'metrics'->'network'->'ip_changes') as ip_changes
        FROM metrics
        WHERE agent_id = ?
        AND data->'metrics'->>'network' IS NOT NULL`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	return r.db.QueryRowContext(ctx, query, agentID).Scan(
		&summary.NetworkMetrics.TotalTraffic,
		&summary.NetworkMetrics.AvgUtilization,
		&summary.NetworkMetrics.IPChanges,
	)
}

// PruneMetrics deletes metrics older than the specified time
func (r *metricsRepository) PruneMetrics(ctx context.Context, before time.Time) error {
	qb := database.NewQueryBuilder(r.db.Driver())
	qb.Raw("DELETE FROM metrics WHERE timestamp < ?", before)

	result, err := r.db.ExecContext(ctx, qb.SQL(), qb.Args()...)
	if err != nil {
		return fmt.Errorf("failed to prune metrics: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	r.logger.Info("Pruned old metrics",
		zap.Int64("deleted_count", affected),
		zap.Time("before", before))

	return nil
}
