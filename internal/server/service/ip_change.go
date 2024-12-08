package service

import (
	"context"
	"fmt"
	"time"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// IPChangeService represents IP change service interface
type IPChangeService interface {
	TrackIPChange(ctx context.Context, agentID string, change *types.IPChange) error
	GetIPChanges(ctx context.Context, agentID string, filter *types.IPChangeFilter) ([]*types.IPChange, error)
	GetIPChangeSummary(ctx context.Context, agentID string) (*types.IPChangeSummary, error)
	GetInterfaceChanges(ctx context.Context, agentID, interfaceName string, since time.Time) ([]*types.IPChange, error)
	AnalyzeChangePatterns(ctx context.Context, agentID string) (*types.IPChangeStats, error)
	CleanupOldChanges(ctx context.Context, before time.Time) error
}

// _ implements IPChangeService
var _ IPChangeService = (*Service)(nil)

// TrackIPChange records and processes an IP change
func (s *Service) TrackIPChange(ctx context.Context, agentID string, change *types.IPChange) error {
	// Verify agent exists
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to find agent: %w", err)
	}

	// Validate change data
	if err := validateIPChange(change); err != nil {
		return fmt.Errorf("invalid IP change: %w", err)
	}

	// Set timestamp if not set
	if change.Timestamp.IsZero() {
		change.Timestamp = time.Now()
	}

	// Save the change
	if err := s.ipChangeRepo.Save(ctx, agentID, change); err != nil {
		return fmt.Errorf("failed to save IP change: %w", err)
	}

	// Send notification
	if s.notifier != nil {
		s.notifier.NotifyIPChange(agent, change)
	}

	s.recordMetric(func(m *types.ServiceMetrics) {
		m.IPChanges++
	})

	s.logger.Info("IP change tracked",
		zap.String("agent_id", agentID),
		zap.String("interface", change.InterfaceName),
		zap.String("action", string(change.Action)),
		zap.Bool("external", change.IsExternal))

	return nil
}

// GetIPChanges retrieves IP changes based on filter
func (s *Service) GetIPChanges(ctx context.Context, agentID string, filter *types.IPChangeFilter) ([]*types.IPChange, error) {
	// Apply default values to filter
	if filter == nil {
		filter = &types.IPChangeFilter{
			StartTime: time.Now().Add(-24 * time.Hour),
			EndTime:   time.Now(),
		}
	}

	if filter.EndTime.IsZero() {
		filter.EndTime = time.Now()
	}

	// Get changes from repository
	changes, err := s.ipChangeRepo.GetRecentChanges(ctx, agentID, filter.StartTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP changes: %w", err)
	}

	// Apply filtering
	return filterIPChanges(changes, filter), nil
}

// GetIPChangeSummary returns a summary of IP changes
func (s *Service) GetIPChangeSummary(ctx context.Context, agentID string) (*types.IPChangeSummary, error) {
	// Get summary from repository
	summary, err := s.ipChangeRepo.GetChangeSummary(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP change summary: %w", err)
	}

	// Get agent status
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Add current status
	summary.CurrentStatus = string(agent.Status)
	summary.LastCheck = agent.LastSeen

	return summary, nil
}

// GetInterfaceChanges returns changes for a specific interface
func (s *Service) GetInterfaceChanges(ctx context.Context, agentID, interfaceName string, since time.Time) ([]*types.IPChange, error) {
	// Verify agent exists
	if _, err := s.GetAgent(ctx, agentID); err != nil {
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	// Get interface changes
	changes, err := s.ipChangeRepo.GetInterfaceChanges(ctx, agentID, interfaceName, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface changes: %w", err)
	}

	return changes, nil
}

// AnalyzeChangePatterns analyzes IP change patterns
func (s *Service) AnalyzeChangePatterns(ctx context.Context, agentID string) (*types.IPChangeStats, error) {
	// Get recent changes for analysis
	changes, err := s.ipChangeRepo.GetRecentChanges(ctx, agentID, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to get changes for analysis: %w", err)
	}

	stats := &types.IPChangeStats{
		TotalChanges: int64(len(changes)),
	}

	if len(changes) > 0 {
		// Calculate change frequencies
		stats.ChangesPerDay = float64(len(changes)) / 30
		stats.ChangesPerWeek = stats.ChangesPerDay * 7
		stats.ChangesPerMonth = float64(len(changes))

		// Analyze patterns
		timeMap := make(map[int]int) // hour -> count
		dayMap := make(map[int]int)  // day -> count
		var totalInterval float64
		lastTime := changes[0].Timestamp

		for _, change := range changes {
			// Track hourly distribution
			timeMap[change.Timestamp.Hour()]++
			// Track daily distribution
			dayMap[int(change.Timestamp.Weekday())]++
			// Calculate interval
			if !lastTime.Equal(change.Timestamp) {
				interval := change.Timestamp.Sub(lastTime).Hours()
				totalInterval += interval
				lastTime = change.Timestamp
			}
		}

		// Find most active periods
		stats.MostActiveHour = findMostActive(timeMap)
		stats.MostActiveDay = findMostActive(dayMap)
		stats.AverageInterval = totalInterval / float64(len(changes)-1)
	}

	return stats, nil
}

// CleanupOldChanges removes old IP change records
func (s *Service) CleanupOldChanges(ctx context.Context, before time.Time) error {
	if err := s.ipChangeRepo.DeleteBefore(ctx, before); err != nil {
		return fmt.Errorf("failed to cleanup old changes: %w", err)
	}

	s.logger.Info("Cleaned up old IP changes",
		zap.Time("before", before))

	return nil
}

// validateIPChange validates IP change data
func validateIPChange(change *types.IPChange) error {
	if change.Version == "" {
		return fmt.Errorf("IP version is required")
	}
	if change.Action == "" {
		return fmt.Errorf("change action is required")
	}
	if !change.IsExternal && change.InterfaceName == "" {
		return fmt.Errorf("interface name is required for non-external changes")
	}
	return nil
}

// filterIPChanges filters IP changes
func filterIPChanges(changes []*types.IPChange, filter *types.IPChangeFilter) []*types.IPChange {
	if filter == nil {
		return changes
	}

	var filtered []*types.IPChange
	for _, change := range changes {
		// Apply filters
		if !change.Timestamp.Before(filter.StartTime) &&
			!change.Timestamp.After(filter.EndTime) &&
			matchesFilter(change, filter) {
			filtered = append(filtered, change)
		}
	}

	return filtered
}

// matchesFilter checks if IP change matches filter
func matchesFilter(change *types.IPChange, filter *types.IPChangeFilter) bool {
	// Check interface filter
	if len(filter.Interfaces) > 0 {
		found := false
		for _, iface := range filter.Interfaces {
			if change.InterfaceName == iface {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check version filter
	if len(filter.Versions) > 0 {
		found := false
		for _, version := range filter.Versions {
			if change.Version == version {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check action filter
	if len(filter.Actions) > 0 {
		found := false
		for _, action := range filter.Actions {
			if string(change.Action) == action {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check external filter
	if filter.IsExternal != nil && change.IsExternal != *filter.IsExternal {
		return false
	}

	return true
}

// findMostActive finds the most active period
func findMostActive(countMap map[int]int) int {
	var maxCount, maxKey int
	for key, count := range countMap {
		if count > maxCount {
			maxCount = count
			maxKey = key
		}
	}
	return maxKey
}
