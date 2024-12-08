package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"wameter/internal/agent/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// AgentService represents agent service interface
type AgentService interface {
	RegisterAgent(ctx context.Context, agent *types.AgentInfo) error
	UpdateAgent(ctx context.Context, agent *types.AgentInfo) error
	GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error)
	GetAgents(ctx context.Context) ([]*types.AgentInfo, error)
	DeleteAgent(ctx context.Context, agentID string) error
	UpdateAgentStatus(ctx context.Context, agentID string, status types.AgentStatus) error
	GetAgentMetrics(ctx context.Context, agentID string) (*types.AgentMetrics, error)
	UpdateAgentConfig(ctx context.Context, agentID string, cfg *config.Config) error
}

// _ implements AgentService
var _ AgentService = (*Service)(nil)

// RegisterAgent registers a new agent
func (s *Service) RegisterAgent(ctx context.Context, agent *types.AgentInfo) error {
	// Validate agent info
	if agent.ID == "" || agent.Hostname == "" {
		return fmt.Errorf("invalid agent info: missing required fields")
	}

	// Check if agent already exists
	existing, err := s.agentRepo.FindByID(ctx, agent.ID)
	if err != nil && !errors.Is(err, types.ErrAgentNotFound) {
		return fmt.Errorf("failed to check existing agent: %w", err)
	}

	// Set initial values
	if agent.RegisteredAt.IsZero() {
		agent.RegisteredAt = time.Now()
	}
	if agent.Status == "" {
		agent.Status = types.AgentStatusOnline
	}
	agent.UpdatedAt = time.Now()
	agent.LastSeen = time.Now()

	if existing != nil {
		// Update agent
		return s.UpdateAgent(ctx, agent)
	}

	// Save to repository
	if err := s.agentRepo.Save(ctx, agent); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	// Update internal state
	s.agentsMu.Lock()
	s.agents[agent.ID] = agent
	s.agentsMu.Unlock()

	s.logger.Info("Agent registered",
		zap.String("id", agent.ID),
		zap.String("hostname", agent.Hostname),
		zap.String("version", agent.Version))

	return nil
}

// UpdateAgent updates existing agent
func (s *Service) UpdateAgent(ctx context.Context, agent *types.AgentInfo) error {
	// Verify agent exists
	existing, err := s.agentRepo.FindByID(ctx, agent.ID)
	if err != nil {
		return fmt.Errorf("failed to find agent: %w", err)
	}

	// Preserve certain fields
	agent.RegisteredAt = existing.RegisteredAt
	agent.UpdatedAt = time.Now()

	// Update in repository
	if err := s.agentRepo.Save(ctx, agent); err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	// Update internal state
	s.agentsMu.Lock()
	s.agents[agent.ID] = agent
	s.agentsMu.Unlock()

	return nil
}

// GetAgent returns agent by ID
func (s *Service) GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error) {
	return s.agentRepo.FindByID(ctx, agentID)
}

// GetAgents returns all agents
func (s *Service) GetAgents(ctx context.Context) ([]*types.AgentInfo, error) {
	return s.agentRepo.List(ctx)
}

// DeleteAgent deletes an agent
func (s *Service) DeleteAgent(ctx context.Context, agentID string) error {
	// Verify agent exists
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	// Check if agent is offline
	if agent.Status == types.AgentStatusOnline {
		return fmt.Errorf("cannot delete online agent")
	}

	// Delete from repository
	if err := s.agentRepo.Delete(ctx, agentID); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	// Remove agent from memory state
	s.agentsMu.Lock()
	delete(s.agents, agentID)
	s.agentsMu.Unlock()

	s.logger.Info("Agent deleted",
		zap.String("id", agentID),
		zap.String("hostname", agent.Hostname))

	return nil
}

// UpdateAgentStatus updates agent status
func (s *Service) UpdateAgentStatus(ctx context.Context, agentID string, status types.AgentStatus) error {
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	// Update status
	agent.Status = status
	agent.UpdatedAt = time.Now()
	if status == types.AgentStatusOnline {
		agent.LastSeen = time.Now()
	}

	// Update in repository
	if err := s.agentRepo.UpdateStatus(ctx, agentID, status); err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	// Update internal state
	s.agentsMu.Lock()
	s.agents[agentID] = agent
	s.agentsMu.Unlock()

	// Send notification if agent went offline
	if status == types.AgentStatusOffline {
		s.notifier.NotifyAgentOffline(agent)
	}

	return nil
}

// GetAgentMetrics returns agent metrics
func (s *Service) GetAgentMetrics(ctx context.Context, agentID string) (*types.AgentMetrics, error) {
	// Get agent
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Get metrics from repository
	metrics, err := s.agentRepo.GetAgentMetrics(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent metrics: %w", err)
	}

	// Add current status
	metrics.CurrentStatus = string(agent.Status)
	metrics.LastSeen = agent.LastSeen

	return metrics, nil
}

// StartAgentMonitoring starts a background task to monitor agent statuses
func (s *Service) StartAgentMonitoring() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAgentStatuses()
		}
	}
}

// UpdateAgentConfig updates agent configuration
func (s *Service) UpdateAgentConfig(ctx context.Context, agentID string, cfg *config.Config) error {
	// Verify agent exists and is online
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	if agent.Status != types.AgentStatusOnline {
		return fmt.Errorf("agent is not online")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Send configuration update command
	cmd := types.Command{
		Type: "config_update",
		Data: cfg,
	}
	if err := s.SendCommand(ctx, agentID, cmd); err != nil {
		return fmt.Errorf("failed to send config update command: %w", err)
	}

	s.logger.Info("Agent configuration updated",
		zap.String("id", agentID),
		zap.String("hostname", agent.Hostname))

	return nil
}

// loadAgents loads existing agents into the service
func (s *Service) loadAgents() {
	const batchSize = 100
	offset := 0
	total := 0

	for {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		agents, err := s.agentRepo.ListWithPagination(ctx, batchSize, offset)
		cancel()
		if err != nil {
			s.logger.Error("Failed to load agents", zap.Error(err))
		}

		if len(agents) == 0 {
			break
		}

		// Update memory state for each agent
		s.agentsMu.Lock()
		for _, agent := range agents {
			if agent.ID == "" || agent.Hostname == "" {
				s.logger.Warn("Skipping invalid agent", zap.String("id", agent.ID))
				continue
			}
			s.agents[agent.ID] = agent
			total++
		}
		s.agentsMu.Unlock()

		offset += len(agents)

		s.logger.Debug("Loaded agents batch",
			zap.Int("batch_size", len(agents)),
			zap.Int("total_loaded", total))

		// Check if context was canceled
		if err := s.ctx.Err(); err != nil {
			s.logger.Error("Agents loading canceled", zap.Error(err))
		}
	}
}

// startAgentMonitoring starts agent monitoring
func (s *Service) startAgentMonitoring() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Agent monitoring stopped")
			return
		case <-ticker.C:
			s.checkAgentStatuses()
		}
	}
}

// // startAgentCacheRefresh starts agent cache refresh
// func (s *Service) startAgentCacheRefresh() {
// 	ticker := time.NewTicker(5 * time.Minute)
// 	defer ticker.Stop()
//
// 	for {
// 		select {
// 		case <-s.ctx.Done():
// 			return
// 		case <-ticker.C:
// 			if err := s.refreshAgentCache(); err != nil {
// 				s.logger.Error("Failed to refresh agent cache", zap.Error(err))
// 			}
// 		}
// 	}
// }
//
// // refreshAgentCache refreshes the agent cache
// func (s *Service) refreshAgentCache() error {
// 	return s.loadAgents()
// }

// checkAgentStatuses checks agent statuses
func (s *Service) checkAgentStatuses() {
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()

	now := time.Now()
	offlineThreshold := 5 * time.Minute

	for id, agent := range s.agents {
		if agent.Status == types.AgentStatusOnline && now.Sub(agent.LastSeen) > offlineThreshold {
			agent.Status = types.AgentStatusOffline

			if err := s.agentRepo.UpdateStatus(context.Background(),
				id, types.AgentStatusOffline); err != nil {
				s.logger.Error("Failed to update agent status",
					zap.Error(err),
					zap.String("agent_id", id))
			}

			s.notifier.NotifyAgentOffline(agent)
		}
	}
}
