package v1

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haiyon/wameter/internal/server/api/response"
	"github.com/haiyon/wameter/internal/server/service"
	"github.com/haiyon/wameter/internal/types"
	"go.uber.org/zap"
)

type API struct {
	service *service.Service
	logger  *zap.Logger
}

func NewAPI(svc *service.Service, logger *zap.Logger) *API {
	return &API{
		service: svc,
		logger:  logger,
	}
}

// RegisterRoutes registers all API routes
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

// Metrics handlers
func (api *API) saveMetrics(c *gin.Context) {
	resp := response.New(c, api.logger)

	var data types.MetricsData
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.BadRequest(errors.New("invalid request body"))
		return
	}

	data.ReportedAt = time.Now()

	if err := api.service.SaveMetrics(c.Request.Context(), &data); err != nil {
		api.logger.Error("Failed to save metrics",
			zap.Error(err),
			zap.String("agent_id", data.AgentID))
		resp.InternalError(errors.New("failed to save metrics"))
		return
	}

	resp.Success(gin.H{"status": "success"})
}

func (api *API) getMetrics(c *gin.Context) {
	resp := response.New(c, api.logger)

	var query struct {
		AgentIDs  []string  `form:"agent_ids"`
		StartTime time.Time `form:"start_time" binding:"required"`
		EndTime   time.Time `form:"end_time" binding:"required"`
		Limit     int       `form:"limit"`
	}

	if err := c.ShouldBindQuery(&query); err != nil {
		resp.BadRequest(errors.New("invalid query parameters"))
		return
	}

	metrics, err := api.service.GetMetrics(c.Request.Context(), service.MetricsQuery{
		AgentIDs:  query.AgentIDs,
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
		Limit:     query.Limit,
	})

	if err != nil {
		api.logger.Error("Failed to get metrics", zap.Error(err))
		resp.InternalError(errors.New("failed to get metrics"))
		return
	}

	resp.Success(metrics)
}

func (api *API) getLatestMetrics(c *gin.Context) {
	resp := response.New(c, api.logger)

	agentID := c.Query("agent_id")
	if agentID == "" {
		resp.BadRequest(errors.New("agent_id is required"))
		return
	}

	metrics, err := api.service.GetLatestMetrics(c.Request.Context(), agentID)
	if err != nil {
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

// Agents handlers
func (api *API) getAgents(c *gin.Context) {
	resp := response.New(c, api.logger)

	agents, err := api.service.GetAgents(c.Request.Context())
	if err != nil {
		api.logger.Error("Failed to get agents", zap.Error(err))
		resp.InternalError(errors.New("failed to get agents"))
		return
	}

	resp.Success(agents)
}

func (api *API) getAgent(c *gin.Context) {
	resp := response.New(c, api.logger)

	agentID := c.Param("id")
	agent, err := api.service.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		if errors.Is(err, types.ErrAgentNotFound) {
			resp.NotFound(errors.New("agent not found"))
			return
		}
		api.logger.Error("Failed to get agent",
			zap.Error(err),
			zap.String("agent_id", agentID))
		resp.InternalError(errors.New("failed to get agent"))
		return
	}

	resp.Success(agent)
}

func (api *API) sendCommand(c *gin.Context) {
	resp := response.New(c, api.logger)

	agentID := c.Param("id")
	var cmd struct {
		Type    string `json:"type" binding:"required"`
		Payload any    `json:"payload"`
	}

	if err := c.ShouldBindJSON(&cmd); err != nil {
		resp.BadRequest(errors.New("invalid command"))
		return
	}

	if err := api.service.SendCommand(c.Request.Context(), agentID, cmd.Type, cmd.Payload); err != nil {
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

	resp.Success(gin.H{"status": "success"})
}

// Health check handler
func (api *API) healthCheck(c *gin.Context) {
	resp := response.New(c, api.logger)

	status := api.service.HealthCheck(c.Request.Context())
	if !status.Healthy {
		resp.Error(http.StatusServiceUnavailable, errors.New("service unhealthy"))
		return
	}
	resp.Success(status)
}
