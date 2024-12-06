package v1

import (
	"context"
	"errors"
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
		agents.POST("/:id/command", api.sendCommand)
	}
}

// getAgents handles retrieving all agents
func (api *API) getAgents(c *gin.Context) {
	resp := response.New(c, api.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

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
	resp := response.New(c, api.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

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

// sendCommand handles sending commands to agents
func (api *API) sendCommand(c *gin.Context) {
	resp := response.New(c, api.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	agentID := c.Param("id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent id is required"))
		return
	}

	var cmd struct {
		Type    string `json:"type" binding:"required"`
		Payload any    `json:"payload"`
	}

	if err := c.ShouldBindJSON(&cmd); err != nil {
		resp.BadRequest(errors.New("invalid command format"))
		return
	}

	// Validate command type
	switch cmd.Type {
	case "config_reload", "collector_restart", "update_agent":
		// Valid commands
	default:
		resp.BadRequest(errors.New("invalid command type"))
		return
	}

	if err := api.service.SendCommand(ctx, agentID, cmd.Type, cmd.Payload); err != nil {
		if errors.Is(err, context.Canceled) {
			api.logger.Info("Client canceled command request",
				zap.String("agent_id", agentID),
				zap.String("command", cmd.Type))
			return
		}

		api.logger.Error("Failed to send command",
			zap.Error(err),
			zap.String("agent_id", agentID),
			zap.String("command", cmd.Type))

		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}

		resp.InternalError(errors.New("failed to send command"))
		return
	}

	resp.Success(gin.H{"status": "success"})
}
