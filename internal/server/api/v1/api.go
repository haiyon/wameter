package v1

import (
	"context"
	"errors"
	"net/http"
	"wameter/internal/server/api/response"
	"wameter/internal/server/config"
	"wameter/internal/server/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// API represents the API
type API struct {
	config  *config.Config
	service *service.Service
	logger  *zap.Logger
}

// NewAPI creates new API
func NewAPI(cfg *config.Config, svc *service.Service, logger *zap.Logger) *API {
	return &API{
		config:  cfg,
		service: svc,
		logger:  logger,
	}
}

// RegisterRoutes registers API routes
func (api *API) RegisterRoutes(r *gin.RouterGroup) {
	// Agents endpoints
	api.RegisterAgentRoutes(r)
	// Metrics endpoints
	api.RegisterMetricsRoutes(r)
	// Health check
	r.GET("/health", api.healthCheck)
}

// healthCheck handles health check requests
func (api *API) healthCheck(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	resp := response.New(c, api.logger)

	status := api.service.HealthCheck(ctx)
	if !status.Healthy {
		resp.Error(http.StatusServiceUnavailable, errors.New("service unhealthy"))
		return
	}

	resp.Success(status)
}
