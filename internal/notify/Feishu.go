package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
func (f *FeishuNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	data := map[string]any{
		"Agent":     agent,
		"Timestamp": time.Now(),
	}
	return f.sendTemplate("agent_offline", data)
}

// NotifyNetworkErrors sends network errors notification
func (f *FeishuNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}
	return f.sendTemplate("network_error", data)
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (f *FeishuNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
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
	return f.sendTemplate("high_utilization", data)
}

// NotifyIPChange sends IP change notification
func (f *FeishuNotifier) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error {
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
	return f.sendTemplate("ip_change", data)
}

// sendTemplate sends notification using template
func (f *FeishuNotifier) sendTemplate(templateName string, data map[string]any) error {
	tmpl, err := f.tplLoader.GetTemplate(ntpl.Feishu, templateName)
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

	return f.send(cardMsg)
}

// send sends message to Feishu
func (f *FeishuNotifier) send(msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	timestamp := time.Now().Unix()
	var webhookURL string
	if f.config.Secret != "" {
		sign := f.generateSignature(timestamp)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", f.config.WebhookURL, timestamp, sign)
	} else {
		webhookURL = f.config.WebhookURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

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
func (f *FeishuNotifier) generateSignature(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, f.config.Secret)
	hmac256 := hmac.New(sha256.New, []byte(f.config.Secret))
	hmac256.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(hmac256.Sum(nil))
}
