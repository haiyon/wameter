package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
	"wameter/internal/config"
	ntpl "wameter/internal/notify/template"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// WeChatNotifier represents WeChat notifier
type WeChatNotifier struct {
	config     *config.WeChatConfig
	logger     *zap.Logger
	client     *http.Client
	token      string
	tokenMu    sync.RWMutex
	tokenTimer *time.Timer
	tplLoader  *ntpl.Loader
}

// WeChatMessage represents WeChat message
type WeChatMessage struct {
	ToUser  string `json:"touser"`
	ToParty string `json:"toparty"`
	ToTag   string `json:"totag"`
	MsgType string `json:"msgtype"`
	AgentID int    `json:"agentid"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Markdown struct {
		Content string `json:"content"`
	} `json:"markdown"`
}

// WeChatTokenResponse represents WeChat token response
type WeChatTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewWeChatNotifier creates a new WeChat notifier
func NewWeChatNotifier(cfg *config.WeChatConfig, loader *ntpl.Loader, logger *zap.Logger) (*WeChatNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("wechat notifier is disabled")
	}

	if cfg.CorpID == "" || cfg.Secret == "" {
		return nil, fmt.Errorf("wechat corpid and secret are required")
	}

	n := &WeChatNotifier{
		config: cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		tplLoader: loader,
	}

	// Get initial token
	if err := n.refreshToken(); err != nil {
		return nil, fmt.Errorf("failed to get initial token: %w", err)
	}

	return n, nil
}

// refreshToken refreshes the WeChat token
func (n *WeChatNotifier) refreshToken() error {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		n.config.CorpID, n.config.Secret)

	resp, err := n.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			n.logger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	var tokenResp WeChatTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.ErrCode != 0 {
		return fmt.Errorf("wechat api error: %s", tokenResp.ErrMsg)
	}

	n.tokenMu.Lock()
	n.token = tokenResp.AccessToken
	n.tokenMu.Unlock()

	// Schedule token refresh
	if n.tokenTimer != nil {
		n.tokenTimer.Stop()
	}
	n.tokenTimer = time.AfterFunc(time.Duration(tokenResp.ExpiresIn)*time.Second*4/5, func() {
		if err := n.refreshToken(); err != nil {
			n.logger.Error("Failed to refresh WeChat token", zap.Error(err))
		}
	})

	return nil
}

// NotifyAgentOffline sends agent offline notification
func (n *WeChatNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	// Prepare data
	data := map[string]any{
		"Agent":     agent,
		"Timestamp": time.Now(),
	}
	return n.sendTemplate("agent_offline", data, "markdown")
}

// NotifyNetworkErrors sends network errors notification
func (n *WeChatNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	// Prepare data
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}
	return n.sendTemplate("network_error", data, "markdown")
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (n *WeChatNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	// Prepare data
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}
	return n.sendTemplate("high_utilization", data, "markdown")
}

// NotifyIPChange sends IP change notification
func (n *WeChatNotifier) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error {
	data := map[string]any{
		"Agent":         agent,
		"Change":        change,
		"Timestamp":     time.Now(),
		"IsExternal":    change.IsExternal,
		"Version":       change.Version,
		"OldAddrs":      change.OldAddrs,
		"NewAddrs":      change.NewAddrs,
		"InterfaceName": change.InterfaceName,
	}
	return n.sendTemplate("ip_change", data, "markdown")
}

// sendTemplate sends WeChat message
func (n *WeChatNotifier) sendTemplate(templateName string, data map[string]any, format ...string) error {
	tmpl, err := n.tplLoader.GetTemplate(ntpl.WeChat, templateName)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Get message format, default to markdown
	// TODO: Add more formats
	messageFormat := "markdown"
	if len(format) > 0 {
		messageFormat = format[0]
	}
	if messageFormat == "markdown" {
		return n.sendMarkdown(content.String())
	}

	return nil
}

// sendMarkdown sends a markdown message
func (n *WeChatNotifier) sendMarkdown(content string) error {
	n.tokenMu.RLock()
	token := n.token
	n.tokenMu.RUnlock()

	msg := WeChatMessage{
		ToUser:  n.config.ToUser,
		ToParty: n.config.ToParty,
		ToTag:   n.config.ToTag,
		MsgType: "markdown",
		AgentID: n.config.AgentID,
	}
	msg.Markdown.Content = content

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := n.client.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			n.logger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		if result.ErrCode == 40014 || result.ErrCode == 42001 {
			// Token expired, refresh and retry
			if err := n.refreshToken(); err != nil {
				return fmt.Errorf("failed to refresh token: %w", err)
			}
			return n.sendMarkdown(content)
		}
		return fmt.Errorf("wechat api error: %s", result.ErrMsg)
	}

	return nil
}

// Health checks the health of the notifier
func (n *WeChatNotifier) Health(_ context.Context) error {
	// Note: Add health check logic here
	return nil
}
