package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
	"wameter/internal/types"
	"wameter/internal/version"

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
	mux.HandleFunc("/v1/command", h.handleCommand)
	mux.HandleFunc("/v1/healthz", h.handleHealthCheck)

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
	if !h.config.Agent.Standalone {
		// Register agent
		if err := h.registerAgent(ctx); err != nil {
			h.logger.Error("Failed to register agent", zap.Error(err))
			return err
		}
	}

	// Start command processor
	h.wg.Add(1)
	go h.processCommands(ctx)

	// Start HTTP server
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if err := h.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			h.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	if !h.config.Agent.Standalone {
		h.wg.Add(1)
		go h.heartbeat(ctx)
	}

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

// registerAgent registers the agent with the server
func (h *Handler) registerAgent(ctx context.Context) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	agent := &types.AgentInfo{
		ID:       h.config.Agent.ID,
		Hostname: hostname,
		Version:  version.GetInfo().Version,
		Port:     h.config.Agent.Port,
		Status:   types.AgentStatusOnline,
	}

	// Build request
	url := fmt.Sprintf("%s/v1/agents", h.config.Agent.Server.Address)
	payload, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf("failed to marshal agent info: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wameter-agent/"+version.GetInfo().Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to register agent: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

// heartbeat handles agent heartbeat
func (h *Handler) heartbeat(ctx context.Context) {
	defer h.wg.Done()

	interval := h.config.Agent.Heartbeat.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failureCount := 0
	maxFailures := h.config.Agent.Heartbeat.MaxFailures
	if maxFailures == 0 {
		maxFailures = 3
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		case <-ticker.C:
			if err := h.sendHeartbeat(ctx); err != nil {
				failureCount++
				h.logger.Error("Failed to send heartbeat",
					zap.Error(err),
					zap.Int("failure_count", failureCount))

				if failureCount >= maxFailures {
					h.logger.Error("Max heartbeat failures reached, attempting to re-register")
					if err := h.registerAgent(ctx); err != nil {
						h.logger.Error("Failed to re-register agent", zap.Error(err))
					} else {
						failureCount = 0
					}
				}
			} else {
				failureCount = 0
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the server
func (h *Handler) sendHeartbeat(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/agents/%s/heartbeat",
		h.config.Agent.Server.Address,
		h.config.Agent.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wameter-agent/"+version.GetInfo().Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed: status=%d body=%s", resp.StatusCode, string(body))
	}
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
