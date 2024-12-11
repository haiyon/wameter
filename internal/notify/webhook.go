package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"wameter/internal/config"
	ntpl "wameter/internal/notify/template"
	"wameter/internal/types"
	"wameter/internal/version"

	"go.uber.org/zap"
)

// WebhookNotifier represents  webhook notifier
type WebhookNotifier struct {
	config    *config.WebhookConfig
	logger    *zap.Logger
	client    *http.Client
	tplLoader *ntpl.Loader
}

// WebhookPayload represents the standard webhook payload structure
type WebhookPayload struct {
	EventType   string    `json:"event_type"`
	EventID     string    `json:"event_id"`
	Timestamp   time.Time `json:"timestamp"`
	Data        any       `json:"data"`
	AgentID     string    `json:"agent_id,omitempty"`
	Hostname    string    `json:"hostname,omitempty"`
	Environment string    `json:"environment,omitempty"`
}

// NewWebhookNotifier creates new webhook notifier
func NewWebhookNotifier(cfg *config.WebhookConfig, loader *ntpl.Loader, logger *zap.Logger) (*WebhookNotifier, error) {
	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 10,
		},
	}

	return &WebhookNotifier{
		config:    cfg,
		logger:    logger,
		client:    client,
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends an agent offline notification
func (n *WebhookNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	payload := WebhookPayload{
		EventType: "agent.offline",
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		AgentID:   agent.ID,
		Hostname:  agent.Hostname,
		Data: map[string]any{
			"status":    agent.Status,
			"last_seen": agent.LastSeen,
			"version":   agent.Version,
			"uptime":    agent.LastSeen.Sub(agent.RegisteredAt).String(),
		},
	}

	return n.sendWebhook(payload)
}

// NotifyNetworkErrors sends a network errors notification
func (n *WebhookNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	payload := WebhookPayload{
		EventType: "network.errors",
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		AgentID:   agentID,
		Data: map[string]any{
			"interface": iface.Name,
			"type":      iface.Type,
			"stats": map[string]uint64{
				"rx_errors":  iface.Statistics.RxErrors,
				"tx_errors":  iface.Statistics.TxErrors,
				"rx_dropped": iface.Statistics.RxDropped,
				"tx_dropped": iface.Statistics.TxDropped,
			},
		},
	}

	return n.sendWebhook(payload)
}

// NotifyHighNetworkUtilization sends a high network utilization notification
func (n *WebhookNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	payload := WebhookPayload{
		EventType: "network.high_utilization",
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		AgentID:   agentID,
		Data: map[string]any{
			"interface": iface.Name,
			"type":      iface.Type,
			"stats": map[string]any{
				"rx_rate":     iface.Statistics.RxBytesRate,
				"tx_rate":     iface.Statistics.TxBytesRate,
				"rx_total":    iface.Statistics.RxBytes,
				"tx_total":    iface.Statistics.TxBytes,
				"utilization": calculateUtilization(iface),
			},
		},
	}

	return n.sendWebhook(payload)
}

// NotifyIPChange sends IP change notification
func (n *WebhookNotifier) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error {
	payload := WebhookPayload{
		EventType: "ip.change",
		EventID:   generateEventID(),
		Timestamp: time.Now(),
		AgentID:   agent.ID,
		Hostname:  agent.Hostname,
		Data: map[string]any{
			"agent":          agent.ID,
			"hostname":       agent.Hostname,
			"interface_name": change.InterfaceName,
			"is_external":    change.IsExternal,
			"version":        change.Version,
			"old_addrs":      change.OldAddrs,
			"new_addrs":      change.NewAddrs,
			"action":         change.Action,
			"reason":         change.Reason,
			"changed_at":     change.Timestamp,
		},
	}

	return n.sendWebhook(payload)
}

// sendWebhook sends a webhook
func (n *WebhookNotifier) sendWebhook(payload WebhookPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Add common data from config
	if n.config.CommonData != nil {
		for k, v := range n.config.CommonData {
			payload.Data.(map[string]any)[k] = v
		}
	}

	// Calculate signature if secret is configured
	signature := ""
	if n.config.Secret != "" {
		signature = calculateSignature(data, []byte(n.config.Secret))
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, n.config.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wameter-webhook/"+version.GetInfo().Version)
	req.Header.Set("X-Wameter-Event", payload.EventType)
	req.Header.Set("X-Wameter-Delivery", payload.EventID)

	if signature != "" {
		req.Header.Set("X-Wameter-Signature", signature)
	}

	// Add custom headers from config
	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}

	// Send request with retry
	var resp *http.Response
	for attempt := 1; attempt <= n.config.MaxRetries; attempt++ {
		resp, err = n.client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}

		if attempt < n.config.MaxRetries {
			time.Sleep(calculateBackoff(attempt))
		}
	}

	if err != nil {
		return fmt.Errorf("failed to send webhook after %d attempts: %w", n.config.MaxRetries, err)
	}

	if resp == nil {
		return fmt.Errorf("failed to send webhook after %d attempts: no response", n.config.MaxRetries)
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			n.logger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook request failed with status %d", resp.StatusCode)
	}

	return nil
}

// generateEventID generates a random event ID
func generateEventID() string {
	return fmt.Sprintf("%d-%x", time.Now().UnixMilli(), randomBytes(4))
}

// calculateSignature calculates the signature
func calculateSignature(payload []byte, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// calculateBackoff calculates the backoff
func calculateBackoff(attempt int) time.Duration {
	backoff := time.Duration(attempt*attempt) * time.Second
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}
	return backoff
}

// calculateUtilization calculates the utilization
func calculateUtilization(iface *types.InterfaceInfo) float64 {
	if iface.Statistics.Speed <= 0 {
		return 0
	}

	totalRate := iface.Statistics.RxBytesRate + iface.Statistics.TxBytesRate
	maxRate := float64(iface.Statistics.Speed * 1000000 / 8) // Convert Mbps to bytes/s

	return (totalRate / maxRate) * 100
}

// randomBytes generates random bytes
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Health checks the health of the notifier
func (n *WebhookNotifier) Health(_ context.Context) error {
	// Note: Add health check logic here
	return nil
}
