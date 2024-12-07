package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"wameter/internal/config"
	ntpl "wameter/internal/notify/template"
	"wameter/internal/types"

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
		Timeout: time.Duration(cfg.Timeout),
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
func (w *WebhookNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
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

	return w.sendWebhook(payload)
}

// NotifyNetworkErrors sends a network errors notification
func (w *WebhookNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
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

	return w.sendWebhook(payload)
}

// NotifyHighNetworkUtilization sends a high network utilization notification
func (w *WebhookNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
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

	return w.sendWebhook(payload)
}

// NotifyIPChange sends IP change notification
func (w *WebhookNotifier) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error {
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

	return w.sendWebhook(payload)
}

// sendWebhook sends a webhook
func (w *WebhookNotifier) sendWebhook(payload WebhookPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Add common data from config
	if w.config.CommonData != nil {
		for k, v := range w.config.CommonData {
			payload.Data.(map[string]any)[k] = v
		}
	}

	// Calculate signature if secret is configured
	signature := ""
	if w.config.Secret != "" {
		signature = calculateSignature(data, []byte(w.config.Secret))
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, w.config.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wameter-webhook/1.0")
	req.Header.Set("X-Wameter-Event", payload.EventType)
	req.Header.Set("X-Wameter-Delivery", payload.EventID)

	if signature != "" {
		req.Header.Set("X-Wameter-Signature", signature)
	}

	// Add custom headers from config
	for k, v := range w.config.Headers {
		req.Header.Set(k, v)
	}

	// Send request with retry
	var resp *http.Response
	for attempt := 1; attempt <= w.config.MaxRetries; attempt++ {
		resp, err = w.client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}

		if attempt < w.config.MaxRetries {
			time.Sleep(calculateBackoff(attempt))
		}
	}

	if err != nil {
		return fmt.Errorf("failed to send webhook after %d attempts: %w", w.config.MaxRetries, err)
	}
	defer resp.Body.Close()

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
