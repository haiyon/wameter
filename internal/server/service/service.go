package service

import (
	"context"
	"sync"
	"time"
	"wameter/internal/database"
	"wameter/internal/server/config"
	"wameter/internal/server/notify"
	"wameter/internal/server/repository"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// Service represents the server service
type Service struct {
	startTime time.Time
	// Core components
	config     *config.Config
	logger     *zap.Logger
	configPath string
	db         database.Interface

	// Repositories
	agentRepo    repository.AgentRepository
	metricsRepo  repository.MetricsRepository
	ipChangeRepo repository.IPChangeRepository

	// Support services
	configMgr *configManager
	notifier  *notify.Manager

	// Command management
	commands map[string]*commandTracker
	history  map[string][]types.CommandHistory

	// State management
	stats struct {
		metricsProcessed int64
		ipChanges        int64
		notifications    int64
		errorCount       int64
		lastError        string
		lastErrorTime    time.Time
	}
	statsMu    sync.RWMutex
	agents     map[string]*types.AgentInfo
	agentsMu   sync.RWMutex
	commandsMu sync.RWMutex

	// Context management
	ctx    context.Context
	cancel context.CancelFunc
}

// NewService creates new service instance
func NewService(cfg *config.Config, db database.Interface, logger *zap.Logger) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	svc := &Service{
		startTime: time.Now(),
		config:    cfg,
		logger:    logger,
		db:        db,
		agents:    make(map[string]*types.AgentInfo),
		commands:  make(map[string]*commandTracker),
		history:   make(map[string][]types.CommandHistory),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Initialize repositories
	svc.initializeRepositories()

	// Initialize notifications
	svc.initializeNotifications()

	// Load existing agents
	svc.loadAgents()

	// Start background tasks
	svc.startBackgroundTasks()

	return svc, nil
}

// Stop stops all service components
func (s *Service) Stop() error {
	// Cancel context first to stop all operations
	s.cancel()

	// Create channel for shutdown completion
	done := make(chan struct{})
	go func() {
		// Stop notification manager
		if s.notifier != nil {
			if err := s.notifier.Stop(); err != nil {
				s.logger.Error("Failed to stop notifier", zap.Error(err))
			}
		}
		// Close database connection
		if s.db != nil {
			if err := s.db.Close(); err != nil {
				s.logger.Error("Failed to close database", zap.Error(err))
			}
		}

		close(done)
	}()

	// Wait for shutdown with timeout
	select {
	case <-done:
		s.logger.Info("All cleanup tasks completed")
	case <-s.ctx.Done():
		s.logger.Warn("Cleanup tasks timed out")
	}

	return nil
}

// initializeRepositories initializes repositories
func (s *Service) initializeRepositories() {
	// Agents
	s.agentRepo = repository.NewAgentRepository(s.db, s.logger)
	// Metrics
	s.metricsRepo = repository.NewMetricsRepository(s.db, s.logger)
	// Agent IP changes
	s.ipChangeRepo = repository.NewIPChangeRepository(s.db, s.logger)
}

// initializeNotifications initializes notifications
func (s *Service) initializeNotifications() {
	// Initialize notification manager
	if s.config.Notify.Enabled {
		notifier, err := notify.NewManager(s.config.Notify, s.logger)
		if err != nil {
			s.cancel()
			s.logger.Fatal("Failed to initialize notification manager", zap.Error(err))
		}
		s.notifier = notifier
	}
}

// startBackgroundTasks starts all background tasks
func (s *Service) startBackgroundTasks() {
	// Start agent monitoring
	go s.startAgentMonitoring()
	// Start cleanup task
	go s.startCleanupTask()

	// Add other background tasks as needed
}

// startCleanupTask starts the cleanup task
func (s *Service) startCleanupTask() {
	ticker := time.NewTicker(s.config.Database.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Cleanup task stopped")
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.config.Database.MetricsRetention)
			if err := s.db.Cleanup(context.Background(), cutoff); err != nil {
				s.logger.Error("Failed to cleanup old metrics", zap.Error(err))
			}
		}
	}
}

// recordMetric records service metrics
func (s *Service) recordMetric(fn func(*types.ServiceMetrics)) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	metrics := &types.ServiceMetrics{
		MetricsProcessed: s.stats.metricsProcessed,
		IPChanges:        s.stats.ipChanges,
		Notifications:    s.stats.notifications,
		ErrorCount:       s.stats.errorCount,
	}

	fn(metrics)

	s.stats.metricsProcessed = metrics.MetricsProcessed
	s.stats.ipChanges = metrics.IPChanges
	s.stats.notifications = metrics.Notifications
	s.stats.errorCount = metrics.ErrorCount
}
