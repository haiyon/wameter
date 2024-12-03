package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"wameter/internal/server/config"
	"wameter/internal/server/notify"
	"wameter/internal/server/storage"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Service represents the server service
type Service struct {
	config   *config.Config
	storage  storage.Storage
	notifier *notify.Manager
	logger   *zap.Logger

	agents     map[string]*types.AgentInfo
	agentsMu   sync.RWMutex
	cleanupCtx context.Context
	cleanupFn  context.CancelFunc
}

// NewService creates new service instance
func NewService(cfg *config.Config, store storage.Storage, logger *zap.Logger) (*Service, error) {
	// Initialize notifier
	notifier, err := notify.NewManager(cfg.Notify, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize notifier: %w", err)
	}

	cleanupCtx, cleanupFn := context.WithCancel(context.Background())

	svc := &Service{
		config:     cfg,
		storage:    store,
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
	s.updateAgentStatus(data.AgentID, types.AgentStatusOnline)

	// Save metrics
	if err := s.storage.SaveMetrics(ctx, data); err != nil {
		return fmt.Errorf("failed to save metrics: %w", err)
	}

	// Process metrics for notifications
	go s.processMetricsNotifications(data)

	return nil
}

// GetMetrics retrieves metrics
func (s *Service) GetMetrics(ctx context.Context, query MetricsQuery) ([]*types.MetricsData, error) {
	storageQuery := &storage.MetricsQuery{
		AgentIDs:  query.AgentIDs,
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
		Limit:     query.Limit,
		OrderBy:   "timestamp",
		Order:     "DESC",
	}

	return s.storage.GetMetrics(ctx, storageQuery, storage.QueryOptions{Timeout: 10 * time.Second})
}

// GetLatestMetrics retrieves the latest metrics
func (s *Service) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	// Query last hour of metrics
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	metrics, err := s.storage.GetMetrics(ctx, &storage.MetricsQuery{
		AgentIDs:  []string{agentID},
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     1,
		OrderBy:   "timestamp",
		Order:     "DESC",
	}, storage.QueryOptions{Timeout: 10 * time.Second})

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
	return s.storage.GetAgents(ctx)
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

	// Check storage
	if err := s.checkStorageHealth(ctx); err != nil {
		status.Healthy = false
		status.Details = append(status.Details, HealthDetail{
			Component: "storage",
			Status:    "unhealthy",
			Error:     err.Error(),
		})
	}

	return status
}

// Internal methods
func (s *Service) startCleanupTask() {
	ticker := time.NewTicker(s.config.Storage.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupCtx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.config.Storage.MetricsRetention)
			if err := s.storage.Cleanup(context.Background(), cutoff); err != nil {
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
				s.storage.UpdateAgentStatus(context.Background(), id, types.AgentStatusOffline)

				// Notify about agent going offline
				s.notifier.NotifyAgentOffline(agent)
			}
		}
	}
}

// updateAgentStatus updates the status of an agent
func (s *Service) updateAgentStatus(agentID string, status types.AgentStatus) {
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()

	agent, exists := s.agents[agentID]
	if !exists {
		hostname := "" // Hostname will be updated from metrics
		agent = &types.AgentInfo{
			ID:           agentID,
			Hostname:     hostname,
			Status:       status,
			RegisteredAt: time.Now(),
			LastSeen:     time.Now(),
			UpdatedAt:    time.Now(),
			Version:      "unknown", // Version will be updated from metrics
		}
		s.agents[agentID] = agent

		// Register agent
		go func() {
			if err := s.storage.RegisterAgent(context.Background(), agent); err != nil {
				s.logger.Error("Failed to register agent",
					zap.Error(err),
					zap.String("agent_id", agentID))
			}
		}()
	} else {
		agent.Status = status
		agent.LastSeen = time.Now()
		agent.UpdatedAt = time.Now()
	}

	// Update storage asynchronously
	go func() {
		if err := s.storage.UpdateAgentStatus(context.Background(), agentID, status); err != nil {
			if !errors.Is(err, types.ErrAgentNotFound) {
				s.logger.Error("Failed to update agent status",
					zap.Error(err),
					zap.String("agent_id", agentID))
			}
		}
	}()
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

// checkStorageHealth checks the storage health
func (s *Service) checkStorageHealth(ctx context.Context) error {
	// Simple storage health check
	_, err := s.storage.GetAgents(ctx)
	return err
}

// Stop stops the service and cleanup resources
func (s *Service) Stop() error {
	s.cleanupFn()
	return s.storage.Close()
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
