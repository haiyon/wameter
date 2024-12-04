package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"wameter/internal/agent/collector"
	"wameter/internal/agent/config"

	"go.uber.org/zap"
)

// Handler handles agent commands and HTTP endpoints
type Handler struct {
	config     *config.Config
	logger     *zap.Logger
	server     *http.Server
	commands   chan Command
	stopChan   chan struct{}
	wg         sync.WaitGroup
	collectors map[string]collector.Collector
	manager    *collector.Manager
}

// NewHandler creates new Handler instance
func NewHandler(cfg *config.Config, logger *zap.Logger, cm *collector.Manager) *Handler {
	h := &Handler{
		config:     cfg,
		logger:     logger,
		commands:   make(chan Command, 100),
		stopChan:   make(chan struct{}),
		collectors: make(map[string]collector.Collector),
		manager:    cm,
	}

	// Create HTTP server for receiving commands
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/command", h.handleCommand)
	mux.HandleFunc("/api/v1/healthz", h.handleHealthCheck)

	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Agent.Port),
		Handler: mux,
	}

	return h
}

// RegisterCollector registers collector with the handler
func (h *Handler) RegisterCollector(name string, c collector.Collector) error {
	if _, exists := h.collectors[name]; exists {
		return fmt.Errorf("collector %s already registered", name)
	}
	h.collectors[name] = c
	return nil
}

// Start begins handling commands and HTTP requests
func (h *Handler) Start(ctx context.Context) error {
	// Start command processor
	h.wg.Add(1)
	go h.processCommands(ctx)

	// Start HTTP server
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
			h.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	return nil
}

// Stop stops the handler
func (h *Handler) Stop() error {
	close(h.stopChan)

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	h.wg.Wait()
	return nil
}

// handleCommand handles incoming command requests
func (h *Handler) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate command before processing
	if err := h.validateCommand(cmd); err != nil {
		http.Error(w, fmt.Sprintf("Invalid command: %v", err), http.StatusBadRequest)
		return
	}

	select {
	case h.commands <- cmd:
		resp := CommandResponse{Success: true}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		resp := CommandResponse{
			Success: false,
			Error:   "Command buffer is full",
		}
		w.WriteHeader(http.StatusServiceUnavailable)

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

// validateCommand validates the incoming command
func (h *Handler) validateCommand(cmd Command) error {
	switch cmd.Type {
	case "config_reload", "collector_restart", "update_agent":
		return nil
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// executeCommand executes the given command
func (h *Handler) executeCommand(ctx context.Context, cmd Command) error {
	h.logger.Info("Executing command", zap.String("type", cmd.Type))

	switch cmd.Type {
	case "config_reload":
		return h.handleConfigReload(ctx, cmd)
	case "collector_restart":
		return h.handleCollectorRestart(ctx, cmd)
	case "update_agent":
		return h.handleUpdateAgent(ctx, cmd)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// processCommands processes commands from the command channel
func (h *Handler) processCommands(ctx context.Context) {
	defer h.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		case cmd := <-h.commands:
			if err := h.executeCommand(ctx, cmd); err != nil {
				h.logger.Error("Failed to execute command",
					zap.String("type", cmd.Type),
					zap.Error(err))
			}
		}
	}
}

// handleHealthCheck handles health check requests
func (h *Handler) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := struct {
		Status    string    `json:"status"`
		Uptime    string    `json:"uptime"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Status:    "healthy",
		Uptime:    time.Since(h.manager.StartTime()).String(),
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
