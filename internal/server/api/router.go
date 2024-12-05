package api

import (
	"net/http"
	"wameter/internal/server/api/middleware"
	av1 "wameter/internal/server/api/v1"
	"wameter/internal/server/config"
	"wameter/internal/server/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Router handles all routing logic
type Router struct {
	engine *gin.Engine
	config *config.Config
	logger *zap.Logger
}

// NewRouter creates and configures a new router
func NewRouter(cfg *config.Config, svc *service.Service, logger *zap.Logger) *Router {
	// Set gin mode based on config
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := &Router{
		engine: gin.New(),
		config: cfg,
		logger: logger,
	}

	// Initialize middleware
	r.setupMiddleware()

	// Initialize API versions
	r.setupAPIV1(svc)

	return r
}

// Handler returns the HTTP handler
func (r *Router) Handler() http.Handler {
	return r.engine
}

// ServeHTTP implements the http.Handler interface
// func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
// 	r.engine.ServeHTTP(w, req)
// }

// setupMiddleware configures all middleware
func (r *Router) setupMiddleware() {
	m := middleware.New(r.config, r.logger)

	// Basic middleware
	r.engine.Use(m.RequestID())
	r.engine.Use(m.Logger())
	r.engine.Use(m.Recovery())

	// Security middleware
	r.engine.Use(m.Secure())

	// CORS if enabled
	if r.config.API.CORS.Enabled {
		r.engine.Use(m.Cors())
	}

	// Rate limiting if enabled
	if r.config.API.RateLimit.Enabled {
		r.engine.Use(m.RateLimit())
	}
}

// setupAPIV1 configures v1 API routes
func (r *Router) setupAPIV1(svc *service.Service) {
	api := av1.NewAPI(svc, r.logger)

	// Create v1 route group
	v1Router := r.engine.Group("/api/v1")

	// Add authentication for protected routes
	if r.config.API.Auth.Enabled {
		m := middleware.New(r.config, r.logger)
		v1Router.Use(m.Auth())
	}

	// Register routes
	api.RegisterRoutes(v1Router)
}
