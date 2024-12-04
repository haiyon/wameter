package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
	ntpl "wameter/internal/server/notify/template"

	"wameter/internal/server/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// DingTalkNotifier represents DingTalk notifier
type DingTalkNotifier struct {
	config    *config.DingTalkConfig
	logger    *zap.Logger
	client    *http.Client
	tplLoader *ntpl.Loader
}

// DingMessage represents DingTalk message
type DingMessage struct {
	MsgType  string       `json:"msgtype"`
	Markdown DingMarkdown `json:"markdown"`
	At       DingAt       `json:"at"`
}

// DingMarkdown represents DingTalk markdown
type DingMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// DingAt represents DingTalk at
type DingAt struct {
	AtMobiles []string `json:"atMobiles"`
	AtUserIds []string `json:"atUserIds"`
	IsAtAll   bool     `json:"isAtAll"`
}

// NewDingTalkNotifier creates a new DingTalk notifier
func NewDingTalkNotifier(cfg *config.DingTalkConfig, loader *ntpl.Loader, logger *zap.Logger) (*DingTalkNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("dingtalk notifier is disabled")
	}

	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("dingtalk access token is required")
	}

	return &DingTalkNotifier{
		config: cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends agent offline notification
func (d *DingTalkNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	// Get template
	tmpl, err := d.tplLoader.GetTemplate(ntpl.DingTalk, "agent_offline")
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

	return d.send("Agent Offline Alert", content.String())
}

// NotifyNetworkErrors sends network errors notification
func (d *DingTalkNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	// Get template
	tmpl, err := d.tplLoader.GetTemplate(ntpl.DingTalk, "agent_offline")
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
	return d.send("Network Errors Alert", content.String())
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (d *DingTalkNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	// Get template
	tmpl, err := d.tplLoader.GetTemplate(ntpl.DingTalk, "high_utilization")
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

	return d.send("High Network Utilization", content.String())
}

// send sends DingTalk message
func (d *DingTalkNotifier) send(title, content string) error {
	msg := DingMessage{
		MsgType: "markdown",
		Markdown: DingMarkdown{
			Title: title,
			Text:  content,
		},
		At: DingAt{
			AtMobiles: d.config.AtMobiles,
			AtUserIds: d.config.AtUserIds,
			IsAtAll:   d.config.AtAll,
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Generate signature if secret is configured
	webhook := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", d.config.AccessToken)
	if d.config.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := d.generateSignature(timestamp)
		webhook = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhook, timestamp, url.QueryEscape(sign))
	}

	resp, err := d.client.Post(webhook, "application/json", bytes.NewBuffer(payload))
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
		return fmt.Errorf("dingtalk api error: %s", result.ErrMsg)
	}

	return nil
}

// generateSignature generates signature
func (d *DingTalkNotifier) generateSignature(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, d.config.Secret)
	hmac256 := hmac.New(sha256.New, []byte(d.config.Secret))
	hmac256.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(hmac256.Sum(nil))
}
