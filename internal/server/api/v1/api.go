package v1

import (
	"context"
	"errors"
	"net/http"
	"time"
	"wameter/internal/server/api/response"
	"wameter/internal/server/service"

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
	// Agents endpoints
	api.RegisterAgentRoutes(r)
	// Metrics endpoints
	api.RegisterMetricsRoutes(r)
	// Health check
	r.GET("/health", api.healthCheck)
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
