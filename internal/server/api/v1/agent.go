package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
	"wameter/internal/server/api/response"
	"wameter/internal/types"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AgentAPI represents agent API
type AgentAPI interface {
	RegisterAgentRoutes(r *gin.RouterGroup)
}

// _ implements AgentAPI
var _ AgentAPI = (*API)(nil)

// RegisterAgentRoutes registers agent routes
func (api *API) RegisterAgentRoutes(r *gin.RouterGroup) {
	// Agents endpoints
	agents := r.Group("/agents")
	{
		agents.GET("", api.getAgents)
		agents.GET("/:id", api.getAgent)
		agents.POST("", api.registerAgent)
		agents.PUT("/:id", api.updateAgent)
		agents.GET("/:id/metrics", api.getAgentMetrics)
		agents.POST("/:id/command", api.sendCommand)
		agents.POST("/:id/heartbeat", api.handleAgentHeartbeat)
	}
}

// getAgents handles retrieving all agents
func (api *API) getAgents(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	agents, err := api.service.GetAgents(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			api.logger.Info("Client canceled agents request")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			resp.Error(http.StatusGatewayTimeout, errors.New("request timeout"))
			return
		}

		api.logger.Error("Failed to get agents",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		resp.InternalError(errors.New("failed to get agents"))
		return
	}

	resp.Success(agents)
}

// getAgent handles retrieving a specific agent
func (api *API) getAgent(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	agentID := c.Param("id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent id is required"))
		return
	}

	agent, err := api.service.GetAgent(ctx, agentID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			api.logger.Info("Client canceled agent request",
				zap.String("agent_id", agentID))
			return
		}

		api.logger.Error("Failed to get agent",
			zap.Error(err),
			zap.String("agent_id", agentID))

		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}

		resp.InternalError(errors.New("failed to get agent"))
		return
	}

	resp.Success(agent)
}

// registerAgent handles agent registration
func (api *API) registerAgent(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	var agent types.AgentInfo
	if err := c.ShouldBindJSON(&agent); err != nil {
		resp.BadRequest(fmt.Errorf("invalid agent data: %w", err))
		return
	}

	if err := api.service.RegisterAgent(ctx, &agent); err != nil {
		api.logger.Error("Failed to register agent",
			zap.Error(err),
			zap.String("agent_id", agent.ID))
		resp.InternalError(fmt.Errorf("failed to register agent"))
		return
	}

	resp.Created(agent)
}

// updateAgent handles agent update requests
func (api *API) updateAgent(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	// Get agent ID from URL
	agentID := c.Param("id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent id is required"))
		return
	}

	// Parse request body
	var update struct {
		Hostname string            `json:"hostname"`
		Version  string            `json:"version"`
		Status   types.AgentStatus `json:"status"`
		Port     int               `json:"port"`
		Tags     map[string]string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&update); err != nil {
		resp.BadRequest(fmt.Errorf("invalid update data: %w", err))
		return
	}

	// Get existing agent
	agent, err := api.service.GetAgent(ctx, agentID)
	if err != nil {
		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}
		resp.InternalError(errors.New("failed to get agent"))
		return
	}

	// Update fields
	if update.Hostname != "" {
		agent.Hostname = update.Hostname
	}
	if update.Version != "" {
		agent.Version = update.Version
	}
	if update.Status != "" {
		agent.Status = update.Status
	}
	if update.Port > 0 {
		agent.Port = update.Port
	}

	// Update agent
	if err := api.service.UpdateAgent(ctx, agent); err != nil {
		api.logger.Error("Failed to update agent",
			zap.Error(err),
			zap.String("agent_id", agentID))
		resp.InternalError(errors.New("failed to update agent"))
		return
	}

	resp.Success(agent)
}

// handleAgentHeartbeat handles agent heartbeat
func (api *API) handleAgentHeartbeat(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)
	agentID := c.Param("id")

	if err := api.service.UpdateAgentStatus(ctx, agentID, types.AgentStatusOnline); err != nil {
		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}
		api.logger.Error("Failed to update agent status",
			zap.Error(err),
			zap.String("agent_id", agentID))
		resp.InternalError(errors.New("failed to update agent status"))
		return
	}

	resp.Success(gin.H{
		"status":    "ok",
		"timestamp": time.Now(),
	})
}

// getAgentMetrics handles agent metrics requests
func (api *API) getAgentMetrics(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	// Get agent ID from URL
	agentID := c.Param("id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent id is required"))
		return
	}

	// Get metrics
	metrics, err := api.service.GetAgentMetrics(ctx, agentID)
	if err != nil {
		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}
		api.logger.Error("Failed to get agent metrics",
			zap.Error(err),
			zap.String("agent_id", agentID))
		resp.InternalError(errors.New("failed to get agent metrics"))
		return
	}

	resp.Success(metrics)
}

// sendCommand handles agent command requests
func (api *API) sendCommand(c *gin.Context) {

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	// Get agent ID from URL
	agentID := c.Param("id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent id is required"))
		return
	}

	// Parse command
	var cmd struct {
		Type    string          `json:"type" binding:"required"`
		Timeout time.Duration   `json:"timeout"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := c.ShouldBindJSON(&cmd); err != nil {
		resp.BadRequest(fmt.Errorf("invalid command format: %w", err))
		return
	}

	// Validate command type
	switch cmd.Type {
	case "config_reload", "collector_restart", "update_agent":
		// Valid commands
	default:
		resp.BadRequest(fmt.Errorf("unsupported command type: %s", cmd.Type))
		return
	}

	// Create command with timeout
	command := types.Command{
		ID:        fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		Type:      cmd.Type,
		Data:      cmd.Payload,
		CreatedAt: time.Now(),
	}

	if cmd.Timeout > 0 {
		command.Timeout = cmd.Timeout
	} else {
		command.Timeout = 30 * time.Second // Default timeout
	}

	// Send command
	if err := api.service.SendCommand(ctx, agentID, command); err != nil {
		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}
		api.logger.Error("Failed to send command",
			zap.Error(err),
			zap.String("agent_id", agentID),
			zap.String("command", cmd.Type))
		resp.InternalError(errors.New("failed to send command"))
		return
	}

	resp.Success(gin.H{
		"command_id": command.ID,
		"status":     "sent",
	})
}
