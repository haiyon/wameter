package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/haiyon/wameter/internal/agent/config"
	"go.uber.org/zap"
)

type Command struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type CommandHandler func(context.Context, Command) error

type CommandResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CommandPayload represents the standard command payload structure
type CommandPayload struct {
	Args map[string]interface{} `json:"args"`
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleConfigReload implements configuration reload command
func (h *Handler) handleConfigReload(ctx context.Context, cmd Command) error {
	var payload CommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return fmt.Errorf("invalid command payload: %w", err)
	}

	configPath, _ := payload.Args["config_path"].(string)
	if configPath == "" {
		configPath = "/etc/wameter/agent.yaml" // default path
	}

	// Load new configuration
	newConfig, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	// Validate new configuration
	if err := validateNewConfig(newConfig); err != nil {
		return fmt.Errorf("invalid new configuration: %w", err)
	}

	// Backup current config
	if err := backupConfig(configPath); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}

	// Apply new configuration
	h.config = newConfig
	h.logger.Info("Configuration reloaded successfully")

	return nil
}

// handleCollectorRestart implements collector restart command
func (h *Handler) handleCollectorRestart(ctx context.Context, cmd Command) error {
	var payload CommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return fmt.Errorf("invalid command payload: %w", err)
	}

	collectorName, _ := payload.Args["collector"].(string)

	// If collector name is specified, restart only that collector
	if collectorName != "" {
		if collector, exists := h.collectors[collectorName]; exists {
			if err := collector.Stop(); err != nil {
				return fmt.Errorf("failed to stop collector %s: %w", collectorName, err)
			}
			if err := collector.Start(ctx); err != nil {
				return fmt.Errorf("failed to restart collector %s: %w", collectorName, err)
			}
			h.logger.Info("Collector restarted successfully",
				zap.String("collector", collectorName))
			return nil
		}
		return fmt.Errorf("collector not found: %s", collectorName)
	}

	// Restart all collectors
	for name, collector := range h.collectors {
		if err := collector.Stop(); err != nil {
			return fmt.Errorf("failed to stop collector %s: %w", name, err)
		}
		if err := collector.Start(ctx); err != nil {
			return fmt.Errorf("failed to restart collector %s: %w", name, err)
		}
	}

	h.logger.Info("All collectors restarted successfully")
	return nil
}

// handleUpdateAgent implements agent update command
func (h *Handler) handleUpdateAgent(ctx context.Context, cmd Command) error {
	var payload CommandPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return fmt.Errorf("invalid command payload: %w", err)
	}

	version, _ := payload.Args["version"].(string)
	if version == "" {
		return fmt.Errorf("version is required")
	}

	// Fetch update package
	pkg, err := h.fetchUpdate(version)
	if err != nil {
		return fmt.Errorf("failed to fetch update: %w", err)
	}

	// Verify package
	if err := h.verifyUpdate(pkg); err != nil {
		return fmt.Errorf("failed to verify update: %w", err)
	}

	// Apply update
	if err := h.applyUpdate(pkg); err != nil {
		return fmt.Errorf("failed to apply update: %w", err)
	}

	h.logger.Info("Agent updated successfully",
		zap.String("version", version))

	// Schedule restart if needed
	if restart, _ := payload.Args["restart"].(bool); restart {
		go func() {
			time.Sleep(5 * time.Second)
			os.Exit(0) // Service manager will restart the agent
		}()
	}

	return nil
}

// Helper functions

func validateNewConfig(cfg *config.Config) error {
	// Add validation logic here
	return nil
}

func backupConfig(configPath string) error {
	backupPath := configPath + fmt.Sprintf(".backup.%d", time.Now().Unix())
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath, data, 0644)
}

func (h *Handler) fetchUpdate(version string) ([]byte, error) {
	// Add update fetching logic here
	return nil, fmt.Errorf("not implemented")
}

func (h *Handler) verifyUpdate(pkg []byte) error {
	// Add update verification logic here
	return fmt.Errorf("not implemented")
}

func (h *Handler) applyUpdate(pkg []byte) error {
	// Add update application logic here
	return fmt.Errorf("not implemented")
}
