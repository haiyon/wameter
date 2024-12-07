package service

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wameter/internal/server/config"
	"wameter/internal/server/db"
	"wameter/internal/server/notify"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Service represents the server service
type Service struct {
	config   *config.Config
	database db.Interface
	notifier *notify.Manager
	logger   *zap.Logger

	agents    map[string]*types.AgentInfo
	agentsMu  sync.RWMutex
	ctx       context.Context
	cleanupFn context.CancelFunc
}

// NewService creates new service instance
func NewService(cfg *config.Config, db db.Interface, logger *zap.Logger) (*Service, error) {
	// Initialize notifier
	notifier, err := notify.NewManager(cfg.Notify, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize notifier: %w", err)
	}

	ctx, cleanupFn := context.WithCancel(context.Background())

	svc := &Service{
		config:    cfg,
		database:  db,
		notifier:  notifier,
		logger:    logger,
		agents:    make(map[string]*types.AgentInfo),
		ctx:       ctx,
		cleanupFn: cleanupFn,
	}

	// Start background tasks
	go svc.StartAgentMonitoring()
	go svc.startCleanupTask()

	return svc, nil
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

// startCleanupTask starts the cleanup task
func (s *Service) startCleanupTask() {
	ticker := time.NewTicker(s.config.Database.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.config.Database.MetricsRetention)
			if err := s.database.Cleanup(context.Background(), cutoff); err != nil {
				s.logger.Error("Failed to cleanup old metrics", zap.Error(err))
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
