package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"wameter/internal/server/api/response"
	"wameter/internal/server/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Middleware represents middleware manager
type Middleware struct {
	logger *zap.Logger
	config *config.Config
}

// New creates a new middleware manager
func New(cfg *config.Config, logger *zap.Logger) *Middleware {
	return &Middleware{
		logger: logger,
		config: cfg,
	}
}

// RequestID adds request ID to context
func (m *Middleware) RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger logs request details
func (m *Middleware) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		requestID := c.GetString("request_id")

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		m.logger.Info("request completed",
			zap.String("request_id", requestID),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", clientIP),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("error", errorMessage))
	}
}

// Recovery recovers from panics
func (m *Middleware) Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get stack trace
				buf := make([]byte, 2048)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				var errMsg string
				switch e := err.(type) {
				case error:
					errMsg = e.Error()
				case string:
					errMsg = e
				default:
					errMsg = fmt.Sprintf("%v", e)
				}

				m.logger.Error("panic recovered",
					zap.String("error", errMsg),
					zap.String("stack", stackTrace))

				response.New(c, m.logger).Error(http.StatusInternalServerError,
					errors.New("internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Cors handles CORS
func (m *Middleware) Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", strings.Join(m.config.API.CORS.AllowedOrigins, ","))
		c.Header("Access-Control-Allow-Methods", strings.Join(m.config.API.CORS.AllowedMethods, ","))
		c.Header("Access-Control-Allow-Headers", strings.Join(m.config.API.CORS.AllowedHeaders, ","))
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RateLimit implements rate limiting
func (m *Middleware) RateLimit() gin.HandlerFunc {
	type client struct {
		count    int
		lastSeen time.Time
	}

	clients := make(map[string]*client)

	return func(c *gin.Context) {
		if !m.config.API.RateLimit.Enabled {
			c.Next()
			return
		}

		ip := c.ClientIP()
		now := time.Now()

		if cl, exists := clients[ip]; exists {
			if now.Sub(cl.lastSeen) > m.config.API.RateLimit.Window {
				cl.count = 0
				cl.lastSeen = now
			}

			if cl.count >= m.config.API.RateLimit.Requests {
				response.New(c, m.logger).Error(http.StatusTooManyRequests,
					errors.New("rate limit exceeded"))
				c.Abort()
				return
			}

			cl.count++
		} else {
			clients[ip] = &client{count: 1, lastSeen: now}
		}

		c.Next()
	}
}

// Auth handles authentication
func (m *Middleware) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			response.New(c, m.logger).Error(http.StatusUnauthorized,
				errors.New("unauthorized"))
			c.Abort()
			return
		}

		// TODO: Implement token validation

		c.Next()
	}
}

// Metrics collects API metrics
func (m *Middleware) Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// start := time.Now()
		// path := c.Request.URL.Path
		// method := c.Request.Method
		//
		// c.Next()
		//
		// duration := time.Since(start)
		// status := c.Writer.Status()

		// Record metrics (example using prometheus)
		/*
			httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
			httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
		*/
	}
}

// NoCache adds no-cache headers
func (m *Middleware) NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}

// Secure adds security headers
func (m *Middleware) Secure() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		if m.config.Server.TLS.Enabled {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
