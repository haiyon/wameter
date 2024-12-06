package service

import (
	"context"
	"errors"
	"time"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// AgentService represents the agent service
type AgentService interface {
	GetAgents(ctx context.Context) ([]*types.AgentInfo, error)
	GetAgent(ctx context.Context, agentID string) (*types.AgentInfo, error)
}

// _ implements AgentService
var _ AgentService = (*Service)(nil)

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
