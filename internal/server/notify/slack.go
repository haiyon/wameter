package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	ntpl "wameter/internal/server/notify/template"

	"wameter/internal/server/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// SlackNotifier represents Slack notifier
type SlackNotifier struct {
	config    *config.SlackConfig
	logger    *zap.Logger
	client    *http.Client
	tplLoader *ntpl.Loader
}

// SlackMessage represents Slack message
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents Slack attachment
type SlackAttachment struct {
	Color     string       `json:"color"`
	Title     string       `json:"title"`
	Text      string       `json:"text"`
	Fields    []SlackField `json:"fields,omitempty"`
	Footer    string       `json:"footer"`
	Timestamp int64        `json:"ts"`
}

// SlackField represents Slack field
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates new SlackNotifier
func NewSlackNotifier(cfg *config.SlackConfig, loader *ntpl.Loader, logger *zap.Logger) (*SlackNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("slack notifier is disabled")
	}

	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("slack webhook URL is required")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
		},
	}

	return &SlackNotifier{
		config:    cfg,
		logger:    logger,
		client:    client,
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends agent offline notification
func (s *SlackNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	tmpl, err := s.tplLoader.GetTemplate(ntpl.Slack, "agent_offline")
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	data := map[string]any{
		"Agent":     agent,
		"Timestamp": time.Now(),
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var msg SlackMessage
	if err := json.Unmarshal(content.Bytes(), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	msg.Channel = s.config.Channel
	msg.Username = s.config.Username
	msg.IconEmoji = s.config.IconEmoji
	msg.IconURL = s.config.IconURL

	return s.send(msg)
}

// NotifyNetworkErrors sends a network errors notification
func (s *SlackNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	tmpl, err := s.tplLoader.GetTemplate(ntpl.Slack, "network_error")
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var msg SlackMessage
	if err := json.Unmarshal(content.Bytes(), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	msg.Channel = s.config.Channel
	msg.Username = s.config.Username
	msg.IconEmoji = s.config.IconEmoji
	msg.IconURL = s.config.IconURL

	return s.send(msg)
}

// NotifyHighNetworkUtilization sends a high network utilization notification
func (s *SlackNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	tmpl, err := s.tplLoader.GetTemplate(ntpl.Slack, "high_utilization")
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var msg SlackMessage
	if err := json.Unmarshal(content.Bytes(), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	msg.Channel = s.config.Channel
	msg.Username = s.config.Username
	msg.IconEmoji = s.config.IconEmoji
	msg.IconURL = s.config.IconURL

	return s.send(msg)
}

// send sends a slack message
func (s *SlackNotifier) send(msg SlackMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack api error: status code %d", resp.StatusCode)
	}

	return nil
}
