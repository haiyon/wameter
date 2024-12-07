package service

import (
	"context"
	"fmt"
	"time"
	"wameter/internal/server/db"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// MetricsService represents the metrics service
type MetricsService interface {
	SaveMetrics(ctx context.Context, data *types.MetricsData) error
	GetMetrics(ctx context.Context, query MetricsQuery) ([]*types.MetricsData, error)
	GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error)
}

// _ implements MetricsService
var _ MetricsService = (*Service)(nil)

// MetricsQuery represents a query for metrics
type MetricsQuery struct {
	AgentIDs  []string  `json:"agent_ids,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit,omitempty"`
}

// SaveMetrics saves metrics data
func (s *Service) SaveMetrics(ctx context.Context, data *types.MetricsData) error {
	// Update agent status
	s.updateAgentStatus(data, types.AgentStatusOnline)

	// Process IP changes if any
	if data.Metrics.Network != nil && len(data.Metrics.Network.IPChanges) > 0 {
		s.processIPChanges(data)
	}

	// Save metrics
	if err := s.database.SaveMetrics(ctx, data); err != nil {
		return fmt.Errorf("failed to save metrics: %w", err)
	}

	// Process metrics for notifications
	go s.processMetricsNotifications(data)

	return nil
}

// GetMetrics retrieves metrics
func (s *Service) GetMetrics(ctx context.Context, query MetricsQuery) ([]*types.MetricsData, error) {
	databaseQuery := &db.MetricsQuery{
		AgentIDs:  query.AgentIDs,
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
		Limit:     query.Limit,
		OrderBy:   "timestamp",
		Order:     "DESC",
	}

	return s.database.GetMetrics(ctx, databaseQuery, db.QueryOptions{Timeout: 10 * time.Second})
}

// GetLatestMetrics retrieves the latest metrics
func (s *Service) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	// Query last hour of metrics
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	metrics, err := s.database.GetMetrics(ctx, &db.MetricsQuery{
		AgentIDs:  []string{agentID},
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     1,
		OrderBy:   "timestamp",
		Order:     "DESC",
	}, db.QueryOptions{Timeout: 10 * time.Second})

	if err != nil {
		return nil, err
	}

	if len(metrics) == 0 {
		return nil, types.ErrAgentNotFound
	}

	return metrics[0], nil
}

// processIPChanges handles IP changes and sends notifications
func (s *Service) processIPChanges(data *types.MetricsData) {
	s.agentsMu.RLock()
	agent, exists := s.agents[data.AgentID]
	s.agentsMu.RUnlock()

	if !exists {
		s.logger.Error("Failed to process IP changes: agent not found",
			zap.String("agent_id", data.AgentID))
		return
	}

	for _, change := range data.Metrics.Network.IPChanges {
		s.logger.Debug("IP change detected",
			zap.String("agent_id", agent.ID),
			zap.String("hostname", agent.Hostname),
			zap.String("interface", change.InterfaceName),
			zap.String("version", string(change.Version)),
			zap.String("action", string(change.Action)),
			zap.String("reason", change.Reason),
			zap.Bool("is_external", change.IsExternal),
			zap.Time("timestamp", change.Timestamp))

		// Send notification
		s.notifier.NotifyIPChange(agent, &change)

		// Save change to database
		if err := s.database.SaveIPChange(context.Background(), agent.ID, &change); err != nil {
			s.logger.Error("Failed to save IP change",
				zap.Error(err),
				zap.String("agent_id", agent.ID))
		}
	}
}

// processMetricsNotifications processes metrics notifications
func (s *Service) processMetricsNotifications(data *types.MetricsData) {
	// Process network metrics for notifications
	if data.Metrics.Network != nil {
		for _, iface := range data.Metrics.Network.Interfaces {
			if iface.Statistics != nil {
				// Check for high error rates
				if iface.Statistics.RxErrors+iface.Statistics.TxErrors > 100 {
					s.notifier.NotifyNetworkErrors(data.AgentID, iface)
				}

				// Check for high utilization
				if iface.Statistics.RxBytesRate > 100*1024*1024 || // 100 MB/s
					iface.Statistics.TxBytesRate > 100*1024*1024 {
					s.notifier.NotifyHighNetworkUtilization(data.AgentID, iface)
				}
			}
		}
	}
}
