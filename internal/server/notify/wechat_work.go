package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	ntpl "wameter/internal/server/notify/template"

	"wameter/internal/server/config"
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
func (w *WeChatNotifier) refreshToken() error {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		w.config.CorpID, w.config.Secret)

	resp, err := w.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp WeChatTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.ErrCode != 0 {
		return fmt.Errorf("wechat api error: %s", tokenResp.ErrMsg)
	}

	w.tokenMu.Lock()
	w.token = tokenResp.AccessToken
	w.tokenMu.Unlock()

	// Schedule token refresh
	if w.tokenTimer != nil {
		w.tokenTimer.Stop()
	}
	w.tokenTimer = time.AfterFunc(time.Duration(tokenResp.ExpiresIn)*time.Second*4/5, func() {
		if err := w.refreshToken(); err != nil {
			w.logger.Error("Failed to refresh WeChat token", zap.Error(err))
		}
	})

	return nil
}

// NotifyAgentOffline sends agent offline notification
func (w *WeChatNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	// Get template
	tmpl, err := w.tplLoader.GetTemplate(ntpl.WeChat, "agent_offline")
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	// Prepare data
	data := map[string]any{
		"Agent":     agent,
		"Timestamp": time.Now(),
	}

	// Execute template
	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return w.sendMarkdown(content.String())
}

// NotifyNetworkErrors sends network errors notification
func (w *WeChatNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	// Get template
	tmpl, err := w.tplLoader.GetTemplate(ntpl.WeChat, "network_error")
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	// Prepare data
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}

	// Execute template
	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return w.sendMarkdown(content.String())
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (w *WeChatNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	// Get template
	tmpl, err := w.tplLoader.GetTemplate(ntpl.WeChat, "high_utilization")
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	// Prepare data
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}

	// Execute template
	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return w.sendMarkdown(content.String())
}

// sendMarkdown sends a markdown message
func (w *WeChatNotifier) sendMarkdown(content string) error {
	w.tokenMu.RLock()
	token := w.token
	w.tokenMu.RUnlock()

	msg := WeChatMessage{
		ToUser:  w.config.ToUser,
		ToParty: w.config.ToParty,
		ToTag:   w.config.ToTag,
		MsgType: "markdown",
		AgentID: w.config.AgentID,
	}
	msg.Markdown.Content = content

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := w.client.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

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
			if err := w.refreshToken(); err != nil {
				return fmt.Errorf("failed to refresh token: %w", err)
			}
			return w.sendMarkdown(content)
		}
		return fmt.Errorf("wechat api error: %s", result.ErrMsg)
	}

	return nil
}
