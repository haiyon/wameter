package service

import (
	"context"
	"fmt"
	"wameter/internal/types"
)

// CommandService represents the command service
type CommandService interface {
	SendCommand(ctx context.Context, agentID string, cmdType string, payload any) error
}

// _ implements CommandService
var _ CommandService = (*Service)(nil)

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
