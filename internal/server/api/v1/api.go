package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"wameter/internal/utils"

	"wameter/internal/server/api/response"
	"wameter/internal/server/service"
	"wameter/internal/types"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// API represents the API
type API struct {
	service *service.Service
	logger  *zap.Logger
}

// NewAPI creates new API
func NewAPI(svc *service.Service, logger *zap.Logger) *API {
	return &API{
		service: svc,
		logger:  logger,
	}
}

// RegisterRoutes registers API routes
func (api *API) RegisterRoutes(r *gin.RouterGroup) {
	// Metrics endpoints
	metrics := r.Group("/metrics")
	{
		metrics.POST("", api.saveMetrics)
		metrics.GET("", api.getMetrics)
		metrics.GET("/latest", api.getLatestMetrics)
	}

	// Agents endpoints
	agents := r.Group("/agents")
	{
		agents.GET("", api.getAgents)
		agents.GET("/:id", api.getAgent)
		agents.POST("/:id/command", api.sendCommand)
	}

	// Health check
	r.GET("/health", api.healthCheck)
}

// saveMetrics handles saving metrics data
func (api *API) saveMetrics(c *gin.Context) {
	resp := response.New(c, api.logger)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var data types.MetricsData
	if err := c.ShouldBindJSON(&data); err != nil {
		api.logger.Error("Invalid metrics data",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		resp.BadRequest(fmt.Errorf("invalid metrics data format: %v", err))
		return
	}

	// Basic validation
	if data.AgentID == "" {
		resp.BadRequest(errors.New("agent_id is required"))
		return
	}
	if data.Hostname == "" {
		resp.BadRequest(errors.New("hostname is required"))
		return
	}
	if data.Metrics.Network != nil {
		if data.Metrics.Network.AgentID == "" {
			resp.BadRequest(errors.New("network.agent_id is required"))
			return
		}
		if data.Metrics.Network.Hostname == "" {
			resp.BadRequest(errors.New("network.hostname is required"))
			return
		}
	}

	// Set reported time
	data.ReportedAt = time.Now()

	if err := api.service.SaveMetrics(ctx, &data); err != nil {
		if errors.Is(err, context.Canceled) {
			api.logger.Info("Client canceled metrics save request",
				zap.String("agent_id", data.AgentID))
			return
		}

		api.logger.Error("Failed to save metrics",
			zap.Error(err),
			zap.String("agent_id", data.AgentID),
			zap.Time("timestamp", data.Timestamp))
		resp.InternalError(errors.New("failed to save metrics"))
		return
	}

	resp.Success(gin.H{"status": "success"})
}

// getMetrics handles retrieving metrics data
func (api *API) getMetrics(c *gin.Context) {
	resp := response.New(c, api.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var query struct {
		AgentIDs     []string `form:"agent_ids"`
		StartTimeStr string   `form:"start_time" binding:"required"`
		EndTimeStr   string   `form:"end_time" binding:"required"`
		Limit        int      `form:"limit"`
	}

	if err := c.ShouldBindQuery(&query); err != nil {
		api.logger.Error("Invalid query parameters",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		resp.BadRequest(errors.New("start_time and end_time are required"))
		return
	}

	// Parse start and end times
	startTime, err := utils.ParseTime(query.StartTimeStr)
	if err != nil {
		resp.BadRequest(fmt.Errorf("invalid start_time format: %v", err))
		return
	}

	endTime, err := utils.ParseTime(query.EndTimeStr)
	if err != nil {
		resp.BadRequest(fmt.Errorf("invalid end_time format: %v", err))
		return
	}

	// Validate time range
	if endTime.Before(startTime) {
		resp.BadRequest(errors.New("end_time must be after start_time"))
		return
	}

	if endTime.Sub(startTime) > 30*24*time.Hour {
		resp.BadRequest(errors.New("time range cannot exceed 30 days"))
		return
	}

	// Set reasonable defaults
	if query.Limit <= 0 {
		query.Limit = 1000
	} else if query.Limit > 10000 {
		query.Limit = 10000
	}

	metrics, err := api.service.GetMetrics(ctx, service.MetricsQuery{
		AgentIDs:  query.AgentIDs,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     query.Limit,
	})

	if err != nil {
		if errors.Is(err, context.Canceled) {
			api.logger.Info("Client canceled metrics request")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			resp.Error(http.StatusGatewayTimeout, errors.New("request timeout"))
			return
		}

		api.logger.Error("Failed to get metrics",
			zap.Error(err),
			zap.String("start_time", query.StartTimeStr),
			zap.String("end_time", query.EndTimeStr),
			zap.Int("limit", query.Limit))
		resp.InternalError(errors.New("failed to get metrics"))
		return
	}

	resp.Success(metrics)
}

// getLatestMetrics handles retrieving latest metrics for an agent
func (api *API) getLatestMetrics(c *gin.Context) {
	resp := response.New(c, api.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	agentID := c.Query("agent_id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent_id is required"))
		return
	}

	metrics, err := api.service.GetLatestMetrics(ctx, agentID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			api.logger.Info("Client canceled latest metrics request",
				zap.String("agent_id", agentID))
			return
		}

		api.logger.Error("Failed to get latest metrics",
			zap.Error(err),
			zap.String("agent_id", agentID))

		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}

		resp.InternalError(errors.New("failed to get latest metrics"))
		return
	}

	resp.Success(metrics)
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

// healthCheck handles health check requests
func (api *API) healthCheck(c *gin.Context) {
	resp := response.New(c, api.logger)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status := api.service.HealthCheck(ctx)
	if !status.Healthy {
		resp.Error(http.StatusServiceUnavailable, errors.New("service unhealthy"))
		return
	}

	resp.Success(status)
}
