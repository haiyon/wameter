package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	"wameter/internal/agent/config"
	"wameter/internal/types"
	"wameter/internal/version"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CommandService represents command service interface
type CommandService interface {
	SendCommand(ctx context.Context, agentID string, cmd types.Command) error
	GetCommandResult(ctx context.Context, commandID string) (*types.CommandResult, error)
	GetPendingCommands(ctx context.Context, agentID string) ([]types.Command, error)
	CancelCommand(ctx context.Context, commandID string) error
	GetCommandHistory(ctx context.Context, agentID string, limit int) ([]types.CommandHistory, error)
}

// _ implements CommandService
var _ CommandService = (*Service)(nil)

// commandTracker tracks command execution
type commandTracker struct {
	command    types.Command
	result     chan types.CommandResult
	cancelFunc context.CancelFunc
	timeout    time.Duration
}

// SendCommand sends a command to an agent
func (s *Service) SendCommand(ctx context.Context, agentID string, cmd types.Command) error {
	// Verify agent exists and is online
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}
	if agent.Status != types.AgentStatusOnline {
		return fmt.Errorf("agent is not online")
	}

	// Generate command ID if not set
	if cmd.ID == "" {
		cmd.ID = fmt.Sprintf("%s-command-%s", agentID, uuid.New().String())
	}
	if cmd.CreatedAt.IsZero() {
		cmd.CreatedAt = time.Now()
	}

	// Set default timeout if not specified
	if cmd.Timeout == 0 {
		cmd.Timeout = 30 * time.Second
	}

	// Create command context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, cmd.Timeout)

	// Create command tracker
	tracker := &commandTracker{
		command:    cmd,
		result:     make(chan types.CommandResult, 1),
		cancelFunc: cancel,
		timeout:    cmd.Timeout,
	}

	// Store tracker
	s.commandsMu.Lock()
	s.commands[cmd.ID] = tracker
	s.commandsMu.Unlock()

	// Start command monitoring
	go s.monitorCommand(cmdCtx, agentID, cmd)

	// Send command to agent
	if err := s.sendCommandToAgent(cmdCtx, agentID, cmd); err != nil {
		cancel()
		return fmt.Errorf("failed to send command: %w", err)
	}

	s.logger.Debug("Command sent",
		zap.String("command_id", cmd.ID),
		zap.String("agent_id", agentID),
		zap.String("type", cmd.Type))

	return nil
}

// GetCommandResult gets the result of a command
func (s *Service) GetCommandResult(ctx context.Context, commandID string) (*types.CommandResult, error) {
	s.commandsMu.RLock()
	tracker, exists := s.commands[commandID]
	s.commandsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("command not found")
	}

	// Wait for result with context timeout
	select {
	case result := <-tracker.result:
		return &result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetPendingCommands gets pending commands for an agent
func (s *Service) GetPendingCommands(_ context.Context, agentID string) ([]types.Command, error) {
	s.commandsMu.RLock()
	defer s.commandsMu.RUnlock()

	var pending []types.Command
	for _, tracker := range s.commands {
		if tracker.command.Type == "agent_command" {
			cmd := tracker.command
			if data, ok := cmd.Data.(map[string]any); ok {
				if targetID, ok := data["agent_id"].(string); ok && targetID == agentID {
					pending = append(pending, cmd)
				}
			}
		}
	}

	return pending, nil
}

// CancelCommand cancels a pending or running command
func (s *Service) CancelCommand(_ context.Context, commandID string) error {
	s.commandsMu.Lock()
	tracker, exists := s.commands[commandID]
	s.commandsMu.Unlock()

	if !exists {
		return fmt.Errorf("command not found")
	}

	// Cancel command execution
	tracker.cancelFunc()

	// Update command status
	result := types.CommandResult{
		CommandID: commandID,
		Status:    types.CommandStatusCanceled,
		EndTime:   time.Now(),
	}
	tracker.result <- result

	s.logger.Info("Command canceled",
		zap.String("command_id", commandID))

	return nil
}

// GetCommandHistory gets command history for an agent
func (s *Service) GetCommandHistory(_ context.Context, agentID string, limit int) ([]types.CommandHistory, error) {
	s.commandsMu.RLock()
	defer s.commandsMu.RUnlock()

	history, exists := s.history[agentID]
	if !exists {
		return nil, nil
	}

	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}

	return history[len(history)-limit:], nil
}

// monitorCommand monitors command execution and handles timeout
func (s *Service) monitorCommand(ctx context.Context, agentID string, cmd types.Command) {
	tracker := s.commands[cmd.ID]
	var result types.CommandResult

	select {
	case result = <-tracker.result:
		// Command completed normally
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result = types.CommandResult{
				CommandID: cmd.ID,
				AgentID:   agentID,
				Status:    types.CommandStatusTimedOut,
				Error:     "command timed out",
				EndTime:   time.Now(),
			}
		} else {
			result = types.CommandResult{
				CommandID: cmd.ID,
				AgentID:   agentID,
				Status:    types.CommandStatusCanceled,
				Error:     "command canceled",
				EndTime:   time.Now(),
			}
		}
	}

	// Update command history
	s.commandsMu.Lock()
	if _, exists := s.history[agentID]; !exists {
		s.history[agentID] = make([]types.CommandHistory, 0)
	}
	s.history[agentID] = append(s.history[agentID], types.CommandHistory{
		Command:  cmd,
		Result:   result,
		Duration: result.EndTime.Sub(result.StartTime),
	})
	s.commandsMu.Unlock()

	// Cleanup command tracker
	s.cleanupCommand(cmd.ID)
}

// sendCommandToAgent sends command to agent via appropriate channel
func (s *Service) sendCommandToAgent(ctx context.Context, agentID string, cmd types.Command) error {
	switch cmd.Type {
	case "config_update":
		return s.sendConfigUpdate(ctx, agentID, cmd)
	case "collector_restart":
		return s.sendCollectorRestart(ctx, agentID, cmd)
	case "agent_update":
		return s.sendAgentUpdate(ctx, agentID, cmd)
	default:
		return fmt.Errorf("unsupported command type: %s", cmd.Type)
	}
}

// cleanupCommand removes command tracker after completion
func (s *Service) cleanupCommand(commandID string) {
	s.commandsMu.Lock()
	defer s.commandsMu.Unlock()

	if tracker, exists := s.commands[commandID]; exists {
		tracker.cancelFunc()
		delete(s.commands, commandID)
	}
}

// sendConfigUpdate sends config update command
func (s *Service) sendConfigUpdate(ctx context.Context, agentID string, cmd types.Command) error {
	c, ok := cmd.Data.(*config.Config)
	if !ok {
		return fmt.Errorf("invalid config data type")
	}

	// Prepare config update message
	message := struct {
		Type   string         `json:"type"`
		Config *config.Config `json:"config"`
	}{
		Type:   "config_update",
		Config: c,
	}

	return s.sendHTTPCommand(ctx, agentID, message)
}

// sendCollectorRestart sends collector restart command
func (s *Service) sendCollectorRestart(ctx context.Context, agentID string, cmd types.Command) error {
	// Prepare collector restart message
	type RestartOptions struct {
		Collector string `json:"collector,omitempty"`
		Force     bool   `json:"force,omitempty"`
	}

	var opts RestartOptions
	if cmd.Data != nil {
		data, err := json.Marshal(cmd.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal restart options: %w", err)
		}
		if err := json.Unmarshal(data, &opts); err != nil {
			return fmt.Errorf("invalid restart options: %w", err)
		}
	}

	message := struct {
		Type    string         `json:"type"`
		Options RestartOptions `json:"options"`
	}{
		Type:    "collector_restart",
		Options: opts,
	}

	return s.sendHTTPCommand(ctx, agentID, message)
}

// sendAgentUpdate sends agent update command
func (s *Service) sendAgentUpdate(ctx context.Context, agentID string, cmd types.Command) error {
	type UpdateOptions struct {
		Version     string `json:"version"`
		ForceUpdate bool   `json:"force_update,omitempty"`
		Restart     bool   `json:"restart,omitempty"`
	}

	var opts UpdateOptions
	if cmd.Data != nil {
		data, err := json.Marshal(cmd.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal update options: %w", err)
		}
		if err := json.Unmarshal(data, &opts); err != nil {
			return fmt.Errorf("invalid update options: %w", err)
		}
	}

	message := struct {
		Type    string        `json:"type"`
		Options UpdateOptions `json:"options"`
	}{
		Type:    "agent_update",
		Options: opts,
	}

	return s.sendHTTPCommand(ctx, agentID, message)
}

// sendHTTPCommand sends command to agent via HTTP
func (s *Service) sendHTTPCommand(ctx context.Context, agentID string, payload any) error {
	// Get agent
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	// Marshal payload
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal command payload: %w", err)
	}

	// Prepare URL
	url := fmt.Sprintf("http://%s:%d/v1/command", agent.Hostname, agent.Port)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wameter-server/"+version.GetInfo().Version)

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("command failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// HandleCommandResult handles command result
func (s *Service) HandleCommandResult(_ context.Context, agentID string, result types.CommandResult) error {
	s.commandsMu.RLock()
	tracker, exists := s.commands[result.CommandID]
	s.commandsMu.RUnlock()

	if !exists {
		return fmt.Errorf("command not found: %s", result.CommandID)
	}

	// Apply default values to result
	if result.EndTime.IsZero() {
		result.EndTime = time.Now()
	}

	// Update command result
	select {
	case tracker.result <- result:
		s.logger.Debug("Command result received",
			zap.String("command_id", result.CommandID),
			zap.String("agent_id", agentID),
			zap.String("status", string(result.Status)))
	default:
		return fmt.Errorf("result channel closed for command: %s", result.CommandID)
	}

	return nil
}
