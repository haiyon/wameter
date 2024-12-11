package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"wameter/internal/config"
	ntpl "wameter/internal/notify/template"
	"wameter/internal/types"
	"wameter/internal/utils"

	"go.uber.org/zap"
)

// FeishuNotifier represents Feishu notifier
type FeishuNotifier struct {
	config    *config.FeishuConfig
	logger    *zap.Logger
	client    *http.Client
	tplLoader *ntpl.Loader
}

// NewFeishuNotifier creates new Feishu notifier
func NewFeishuNotifier(cfg *config.FeishuConfig, loader *ntpl.Loader, logger *zap.Logger) (*FeishuNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("feishu notifier is disabled")
	}

	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("feishu webhook URL is required")
	}

	return &FeishuNotifier{
		config: cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 5,
			},
		},
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends agent offline notification
func (n *FeishuNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	data := map[string]any{
		"Agent":     agent,
		"Timestamp": time.Now(),
	}
	return n.sendTemplate("agent_offline", data)
}

// NotifyNetworkErrors sends network errors notification
func (n *FeishuNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}
	return n.sendTemplate("network_error", data)
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (n *FeishuNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
		"Stats": map[string]string{
			"RxRate":  utils.FormatBytesRate(iface.Statistics.RxBytesRate),
			"TxRate":  utils.FormatBytesRate(iface.Statistics.TxBytesRate),
			"RxTotal": utils.FormatBytes(iface.Statistics.RxBytes),
			"TxTotal": utils.FormatBytes(iface.Statistics.TxBytes),
		},
	}
	return n.sendTemplate("high_utilization", data)
}

// NotifyIPChange sends IP change notification
func (n *FeishuNotifier) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error {
	data := map[string]any{
		"Agent":         agent,
		"Change":        change,
		"Action":        change.Action,
		"Reason":        change.Reason,
		"IsExternal":    change.IsExternal,
		"Version":       change.Version,
		"OldAddrs":      change.OldAddrs,
		"NewAddrs":      change.NewAddrs,
		"InterfaceName": change.InterfaceName,
		"Timestamp":     time.Now(),
	}
	return n.sendTemplate("ip_change", data)
}

// sendTemplate sends notification using template
func (n *FeishuNotifier) sendTemplate(templateName string, data map[string]any) error {
	tmpl, err := n.tplLoader.GetTemplate(ntpl.Feishu, templateName)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var cardMsg struct {
		MsgType string         `json:"msg_type"`
		Card    map[string]any `json:"card"`
	}
	if err := json.Unmarshal(content.Bytes(), &cardMsg.Card); err != nil {
		return fmt.Errorf("failed to unmarshal template: %w", err)
	}
	cardMsg.MsgType = "interactive"

	return n.send(cardMsg)
}

// send sends message to Feishu
func (n *FeishuNotifier) send(msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	timestamp := time.Now().Unix()
	var webhookURL string
	if n.config.Secret != "" {
		sign := n.generateSignature(timestamp)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", n.config.WebhookURL, timestamp, sign)
	} else {
		webhookURL = n.config.WebhookURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			n.logger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("feishu api error: %s", result.Msg)
	}

	return nil
}

// generateSignature generates signature for Feishu webhook
func (n *FeishuNotifier) generateSignature(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, n.config.Secret)
	hmac256 := hmac.New(sha256.New, []byte(n.config.Secret))
	hmac256.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(hmac256.Sum(nil))
}

// Health checks the health of the notifier
func (n *FeishuNotifier) Health(_ context.Context) error {
	// Note: Add health check logic here
	return nil
}
