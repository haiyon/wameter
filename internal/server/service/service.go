package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"wameter/internal/notify"

	"wameter/internal/server/config"
	"wameter/internal/server/database"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Service represents the server service
type Service struct {
	config   *config.Config
	database database.Database
	notifier *notify.Manager
	logger   *zap.Logger

	agents     map[string]*types.AgentInfo
	agentsMu   sync.RWMutex
	cleanupCtx context.Context
	cleanupFn  context.CancelFunc
}

// NewService creates new service instance
func NewService(cfg *config.Config, store database.Database, logger *zap.Logger) (*Service, error) {
	// Initialize notifier
	notifier, err := notify.NewManager(cfg.Notify, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize notifier: %w", err)
	}

	cleanupCtx, cleanupFn := context.WithCancel(context.Background())

	svc := &Service{
		config:     cfg,
		database:   store,
		notifier:   notifier,
		logger:     logger,
		agents:     make(map[string]*types.AgentInfo),
		cleanupCtx: cleanupCtx,
		cleanupFn:  cleanupFn,
	}

	// Start background tasks
	go svc.startCleanupTask()
	go svc.startAgentMonitoring()

	return svc, nil
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
	databaseQuery := &database.MetricsQuery{
		AgentIDs:  query.AgentIDs,
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
		Limit:     query.Limit,
		OrderBy:   "timestamp",
		Order:     "DESC",
	}

	return s.database.GetMetrics(ctx, databaseQuery, database.QueryOptions{Timeout: 10 * time.Second})
}

// GetLatestMetrics retrieves the latest metrics
func (s *Service) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	// Query last hour of metrics
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	metrics, err := s.database.GetMetrics(ctx, &database.MetricsQuery{
		AgentIDs:  []string{agentID},
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     1,
		OrderBy:   "timestamp",
		Order:     "DESC",
	}, database.QueryOptions{Timeout: 10 * time.Second})

	if err != nil {
		return nil, err
	}

	if len(metrics) == 0 {
		return nil, types.ErrAgentNotFound
	}

	return metrics[0], nil
}

// GetAgents retrieves all agents
func (s *Service) GetAgents(ctx context.Context) ([]*types.AgentInfo, error) {
	// Add timeout if not already set in context
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Use synchronization to prevent concurrent map access
	s.agentsMu.RLock()
	agents := make([]*types.AgentInfo, 0, len(s.agents))
	for _, agent := range s.agents {
		// Check context cancellation during iteration
		select {
		case <-ctx.Done():
			s.agentsMu.RUnlock()
			return nil, ctx.Err()
		default:
			agentCopy := *agent // Create a copy to prevent data races
			agents = append(agents, &agentCopy)
		}
	}
	s.agentsMu.RUnlock()

	if len(agents) == 0 {
		// Consider whether empty result is an error in your case
		return agents, nil
	}

	// Add optional caching if needed
	// s.cacheAgents(agents)

	return agents, nil
}

// GetAgent retrieves an agent
func (s *Service) GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error) {
	s.agentsMu.RLock()
	agent, exists := s.agents[agentID]
	s.agentsMu.RUnlock()

	if !exists {
		return nil, types.ErrAgentNotFound
	}

	return agent, nil
}

// SendCommand sends a command to an agent
func (s *Service) SendCommand(ctx context.Context, agentID string, cmdType string, payload any) error {
	s.agentsMu.RLock()
	agent, exists := s.agents[agentID]
	s.agentsMu.RUnlock()

	if !exists {
		return types.ErrAgentNotFound
	}

	if agent.Status != types.AgentStatusOnline {
		return fmt.Errorf("agent is not online")
	}

	// TODO: Implement command sending logic
	return fmt.Errorf("command sending not implemented")
}

// HealthCheck performs a health check
func (s *Service) HealthCheck(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Healthy:   true,
		Timestamp: time.Now(),
	}

	// Check database
	if err := s.checkDatabaseHealth(ctx); err != nil {
		status.Healthy = false
		status.Details = append(status.Details, HealthDetail{
			Component: "database",
			Status:    "unhealthy",
			Error:     err.Error(),
		})
	}

	return status
}

// Internal methods
func (s *Service) startCleanupTask() {
	ticker := time.NewTicker(s.config.Database.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupCtx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.config.Database.MetricsRetention)
			if err := s.database.Cleanup(context.Background(), cutoff); err != nil {
				s.logger.Error("Failed to cleanup old metrics", zap.Error(err))
			}
		}
	}
}

// startAgentMonitoring starts a background task to monitor agent statuses
func (s *Service) startAgentMonitoring() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupCtx.Done():
			return
		case <-ticker.C:
			s.checkAgentStatuses()
		}
	}
}

// checkAgentStatuses checks agent statuses
func (s *Service) checkAgentStatuses() {
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()

	now := time.Now()
	offlineThreshold := 5 * time.Minute

	for id, agent := range s.agents {
		if agent.Status == types.AgentStatusOnline {
			if now.Sub(agent.LastSeen) > offlineThreshold {
				agent.Status = types.AgentStatusOffline
				if err := s.database.UpdateAgentStatus(context.Background(), id, types.AgentStatusOffline); err != nil {
					s.logger.Error("Failed to update agent status",
						zap.Error(err),
						zap.String("agent_id", id))
				}
				// Notify about agent going offline
				s.notifier.NotifyAgentOffline(agent)
			}
		}
	}
}

// updateAgentStatus updates the status of an agent
func (s *Service) updateAgentStatus(data *types.MetricsData, status types.AgentStatus) {
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()

	now := time.Now()
	agent, exists := s.agents[data.AgentID]
	if !exists {
		agent = &types.AgentInfo{
			ID:           data.AgentID,
			Hostname:     data.Hostname,
			Status:       status,
			RegisteredAt: now,
			LastSeen:     now,
			UpdatedAt:    now,
			Version:      data.Version,
		}
		s.agents[data.AgentID] = agent

		// Register agent
		agentCopy := *agent
		go func(agent types.AgentInfo) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := s.database.RegisterAgent(ctx, &agent); err != nil {
				if !errors.Is(err, types.ErrAgentNotFound) {
					s.logger.Error("Failed to register agent",
						zap.Error(err),
						zap.String("agent_id", data.AgentID))
				}
			}
		}(agentCopy)
	} else {
		agent.Status = status
		agent.LastSeen = now
		agent.UpdatedAt = now
		agent.Hostname = data.Hostname
		agent.Version = data.Version

		// Update database asynchronously
		agentCopy := *agent
		go func(agent types.AgentInfo) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := s.database.UpdateAgentStatus(ctx, data.AgentID, agent.Status); err != nil {
				s.logger.Error("Failed to update agent status",
					zap.Error(err),
					zap.String("agent_id", data.AgentID))
			}
		}(agentCopy)
	}
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

// checkDatabaseHealth checks the database health
func (s *Service) checkDatabaseHealth(ctx context.Context) error {
	// Simple database health check
	_, err := s.database.GetAgents(ctx)
	return err
}

// Stop stops the service and cleanup resources
func (s *Service) Stop() error {
	s.cleanupFn()
	return s.database.Close()
}

// HealthStatus health check
type HealthStatus struct {
	Healthy   bool           `json:"healthy"`
	Timestamp time.Time      `json:"timestamp"`
	Details   []HealthDetail `json:"details,omitempty"`
}

// HealthDetail represents a health detail
type HealthDetail struct {
	Component string `json:"component"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// MetricsQuery represents a query for metrics
type MetricsQuery struct {
	AgentIDs  []string  `json:"agent_ids,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit,omitempty"`
}
