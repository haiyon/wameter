package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"wameter/internal/database"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// ipChangeRepository represents IP change repository implementation
type ipChangeRepository struct {
	db     database.Interface
	logger *zap.Logger
}

// NewIPChangeRepository creates new IP change repository
func NewIPChangeRepository(db database.Interface, logger *zap.Logger) IPChangeRepository {
	return &ipChangeRepository{
		db:     db,
		logger: logger,
	}
}

// Save saves IP change
func (r *ipChangeRepository) Save(ctx context.Context, agentID string, change *types.IPChange) error {
	query := `
        INSERT INTO ip_changes (
            agent_id, interface_name, version,
            is_external, old_addrs, new_addrs,
            action, reason, timestamp, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	oldAddrs, err := json.Marshal(change.OldAddrs)
	if err != nil {
		return fmt.Errorf("failed to marshal old addresses: %w", err)
	}

	newAddrs, err := json.Marshal(change.NewAddrs)
	if err != nil {
		return fmt.Errorf("failed to marshal new addresses: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		agentID,
		change.InterfaceName,
		change.Version,
		change.IsExternal,
		oldAddrs,
		newAddrs,
		change.Action,
		change.Reason,
		change.Timestamp,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save IP change: %w", err)
	}

	return nil
}

// GetRecentChanges returns recent IP changes
func (r *ipChangeRepository) GetRecentChanges(ctx context.Context, agentID string, since time.Time) ([]*types.IPChange, error) {
	query := `
        SELECT interface_name, version, is_external,
               old_addrs, new_addrs, action, reason,
               timestamp, created_at
        FROM ip_changes
        WHERE agent_id = ? AND timestamp > ?
        ORDER BY timestamp DESC`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	rows, err := r.db.QueryContext(ctx, query, agentID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query IP changes: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var changes []*types.IPChange
	for rows.Next() {
		var change types.IPChange
		var oldAddrs, newAddrs []byte
		var createdAt time.Time

		err := rows.Scan(
			&change.InterfaceName,
			&change.Version,
			&change.IsExternal,
			&oldAddrs,
			&newAddrs,
			&change.Action,
			&change.Reason,
			&change.Timestamp,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan IP change: %w", err)
		}

		if err := json.Unmarshal(oldAddrs, &change.OldAddrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal old addresses: %w", err)
		}

		if err := json.Unmarshal(newAddrs, &change.NewAddrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal new addresses: %w", err)
		}

		changes = append(changes, &change)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating IP changes: %w", err)
	}

	return changes, nil
}

// GetChangeSummary returns a summary of IP changes
func (r *ipChangeRepository) GetChangeSummary(ctx context.Context, agentID string) (*types.IPChangeSummary, error) {
	query := `
        SELECT
            COUNT(*) as total_changes,
            COUNT(DISTINCT interface_name) as affected_interfaces,
            COUNT(CASE WHEN is_external THEN 1 END) as external_changes,
            MIN(timestamp) as first_change,
            MAX(timestamp) as last_change
        FROM ip_changes
        WHERE agent_id = ?`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	summary := &types.IPChangeSummary{}
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&summary.TotalChanges,
		&summary.AffectedInterfaces,
		&summary.ExternalChanges,
		&summary.FirstChange,
		&summary.LastChange,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get IP change summary: %w", err)
	}

	// Get change frequency statistics
	if err := r.getChangeFrequencyStats(ctx, agentID, summary); err != nil {
		r.logger.Error("Failed to get change frequency stats",
			zap.Error(err),
			zap.String("agent_id", agentID))
	}

	return summary, nil
}

// getChangeFrequencyStats calculates IP change frequency statistics
func (r *ipChangeRepository) getChangeFrequencyStats(ctx context.Context, agentID string, summary *types.IPChangeSummary) error {
	query := `
        WITH daily_changes AS (
            SELECT
                DATE(timestamp) as change_date,
                COUNT(*) as changes
            FROM ip_changes
            WHERE agent_id = ?
            GROUP BY DATE(timestamp)
        )
        SELECT
            AVG(changes) as avg_daily_changes,
            MAX(changes) as max_daily_changes
        FROM daily_changes`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	return r.db.QueryRowContext(ctx, query, agentID).Scan(
		&summary.AvgDailyChanges,
		&summary.MaxDailyChanges,
	)
}

// DeleteBefore deletes IP changes before the given time
func (r *ipChangeRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	query := "DELETE FROM ip_changes WHERE timestamp < ?"
	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to delete IP changes: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	r.logger.Info("Deleted old IP changes",
		zap.Int64("count", affected),
		zap.Time("before", before))

	return nil
}

// GetInterfaceChanges returns changes for a specific interface
func (r *ipChangeRepository) GetInterfaceChanges(ctx context.Context, agentID, interfaceName string, since time.Time) ([]*types.IPChange, error) {
	query := `
        SELECT version, is_external, old_addrs, new_addrs,
               action, reason, timestamp, created_at
        FROM ip_changes
        WHERE agent_id = ?
        AND interface_name = ?
        AND timestamp > ?
        ORDER BY timestamp DESC`

	if r.db.Driver() == "postgres" {
		query = database.ConvertPlaceholders(query)
	}

	rows, err := r.db.QueryContext(ctx, query, agentID, interfaceName, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query interface changes: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var changes []*types.IPChange
	for rows.Next() {
		var change types.IPChange
		var oldAddrs, newAddrs []byte
		var createdAt time.Time

		err := rows.Scan(
			&change.Version,
			&change.IsExternal,
			&oldAddrs,
			&newAddrs,
			&change.Action,
			&change.Reason,
			&change.Timestamp,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan interface change: %w", err)
		}

		change.InterfaceName = interfaceName

		if err := json.Unmarshal(oldAddrs, &change.OldAddrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal old addresses: %w", err)
		}

		if err := json.Unmarshal(newAddrs, &change.NewAddrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal new addresses: %w", err)
		}

		changes = append(changes, &change)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating interface changes: %w", err)
	}

	return changes, nil
}
