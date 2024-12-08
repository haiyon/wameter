// ./wameter/internal/server/service/health.go

package service

import (
	"context"
	"fmt"
	"runtime"
	"time"
	"wameter/internal/types"
	"wameter/internal/version"

	"go.uber.org/zap"
)

// HealthService represents health check service interface
type HealthService interface {
	HealthCheck(ctx context.Context) *types.HealthStatus
	GetServiceMetrics(ctx context.Context) *types.ServiceMetrics
	GetComponentStatus(ctx context.Context) map[string]*types.ComponentStatus
}

// StartHealthCheck starts periodic health checking
func (s *Service) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				status := s.HealthCheck(ctx)
				if !status.Healthy {
					s.logger.Warn("Unhealthy status detected",
						zap.Any("details", status.Details))

					if s.notifier != nil {
						// Add notification logic here
					}
				}
			}
		}
	}()
}

// StopHealthCheck stops the health check loop
func (s *Service) StopHealthCheck() {
	s.cancel()
	// Add cleanup logic here
}

// HealthCheck performs comprehensive health check
func (s *Service) HealthCheck(ctx context.Context) *types.HealthStatus {
	status := &types.HealthStatus{
		Healthy:   true,
		Timestamp: time.Now(),
		Version:   version.GetInfo().Version,
		StartTime: s.startTime,
		Uptime:    time.Since(s.startTime),
	}

	// Check database health
	if err := s.checkDatabaseHealth(ctx); err != nil {
		status.Healthy = false
		status.Details = append(status.Details, types.ComponentStatus{
			Name:      "database",
			Status:    "unhealthy",
			Error:     err.Error(),
			LastCheck: time.Now(),
		})
	} else {
		status.Details = append(status.Details, types.ComponentStatus{
			Name:      "database",
			Status:    "healthy",
			LastCheck: time.Now(),
		})
	}

	// Check notification service
	if s.notifier != nil {
		if err := s.notifier.Check(ctx); err != nil {
			status.Healthy = false
			status.Details = append(status.Details, types.ComponentStatus{
				Name:      "notifier",
				Status:    "unhealthy",
				Error:     err.Error(),
				LastCheck: time.Now(),
			})
		} else {
			status.Details = append(status.Details, types.ComponentStatus{
				Name:      "notifier",
				Status:    "healthy",
				LastCheck: time.Now(),
			})
		}
	}

	// Check agent monitoring
	s.agentsMu.RLock()
	activeAgents := 0
	for _, agent := range s.agents {
		if agent.Status == types.AgentStatusOnline {
			activeAgents++
		}
	}
	s.agentsMu.RUnlock()

	status.Details = append(status.Details, types.ComponentStatus{
		Name:      "agent_monitoring",
		Status:    "healthy",
		Message:   fmt.Sprintf("Active agents: %d", activeAgents),
		LastCheck: time.Now(),
	})

	return status
}

// GetServiceMetrics returns service metrics
func (s *Service) GetServiceMetrics(_ context.Context) *types.ServiceMetrics {
	metrics := &types.ServiceMetrics{
		StartTime:  s.startTime,
		SystemInfo: s.collectSystemStats(),
	}

	// Get database stats
	if dbStats := s.getDatabaseStats(); dbStats != nil {
		metrics.DatabaseStats = *dbStats
	}

	// Get agent stats
	s.agentsMu.RLock()
	metrics.ActiveAgents = 0
	metrics.TotalAgents = len(s.agents)
	for _, agent := range s.agents {
		if agent.Status == types.AgentStatusOnline {
			metrics.ActiveAgents++
		}
	}
	s.agentsMu.RUnlock()

	// Get metrics from internal counters
	s.statsMu.RLock()
	metrics.MetricsProcessed = s.stats.metricsProcessed
	metrics.IPChanges = s.stats.ipChanges
	metrics.Notifications = s.stats.notifications
	metrics.ErrorCount = s.stats.errorCount
	metrics.LastError = s.stats.lastError
	metrics.LastErrorTime = s.stats.lastErrorTime
	s.statsMu.RUnlock()

	return metrics
}

// GetComponentStatus returns detailed component status
func (s *Service) GetComponentStatus(ctx context.Context) map[string]*types.ComponentStatus {
	statuses := make(map[string]*types.ComponentStatus)

	// Check database
	dbStatus := &types.ComponentStatus{
		Name:      "database",
		LastCheck: time.Now(),
	}
	if err := s.checkDatabaseHealth(ctx); err != nil {
		dbStatus.Status = "unhealthy"
		dbStatus.Error = err.Error()
	} else {
		dbStatus.Status = "healthy"
	}
	statuses["database"] = dbStatus

	// Check notifier
	if s.notifier != nil {
		notifierStatus := &types.ComponentStatus{
			Name:      "notifier",
			LastCheck: time.Now(),
		}
		if err := s.notifier.Check(ctx); err != nil {
			notifierStatus.Status = "unhealthy"
			notifierStatus.Error = err.Error()
		} else {
			notifierStatus.Status = "healthy"
		}
		statuses["notifier"] = notifierStatus
	}

	// Check agent monitoring
	monitoringStatus := &types.ComponentStatus{
		Name:      "agent_monitoring",
		LastCheck: time.Now(),
	}
	s.agentsMu.RLock()
	activeAgents := 0
	for _, agent := range s.agents {
		if agent.Status == types.AgentStatusOnline {
			activeAgents++
		}
	}
	s.agentsMu.RUnlock()
	monitoringStatus.Status = "healthy"
	monitoringStatus.Message = fmt.Sprintf("Active agents: %d", activeAgents)
	statuses["agent_monitoring"] = monitoringStatus

	// Add system metrics
	sysStats := s.collectSystemStats()
	systemStatus := &types.ComponentStatus{
		Name:      "system",
		Status:    "healthy",
		LastCheck: time.Now(),
		Message: fmt.Sprintf("Goroutines: %d, Memory: %dMB, GC: %d",
			sysStats.NumGoroutine,
			sysStats.MemStats.Alloc/1024/1024,
			sysStats.NumGC),
	}
	statuses["system"] = systemStatus

	return statuses
}

// collectSystemStats collects system statistics
func (s *Service) collectSystemStats() *types.SystemStats {
	stats := &types.SystemStats{}
	stats.NumGoroutine = runtime.NumGoroutine()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	stats.MemStats = memStats
	stats.LastGC = time.Unix(0, int64(memStats.LastGC))
	stats.NumGC = memStats.NumGC

	// Get CPU usage (simplified)
	stats.CPUUsage = 0.0 // TODO: Implement CPU usage calculation

	return stats
}

// getDatabaseStats returns database statistics
func (s *Service) getDatabaseStats() *types.DatabaseStats {
	if s.db == nil {
		return nil
	}

	stats := s.db.Stats()
	return &types.DatabaseStats{
		OpenConnections: stats.OpenConnections,
		InUse:           stats.InUse,
		Idle:            stats.Idle,
		WaitCount:       stats.WaitCount,
		QueryCount:      stats.QueryCount,
		ErrorCount:      stats.QueryErrors,
		SlowQueries:     stats.SlowQueries,
		AvgQueryTime:    stats.AvgQueryTime,
	}
}

// checkDatabaseHealth verifies database connectivity
func (s *Service) checkDatabaseHealth(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	return s.db.Ping(ctx)
}
