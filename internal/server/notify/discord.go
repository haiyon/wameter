package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	ntpl "wameter/internal/server/notify/template"

	"wameter/internal/server/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// DiscordNotifier represents Discord notifier
type DiscordNotifier struct {
	config    *config.DiscordConfig
	logger    *zap.Logger
	client    *http.Client
	tplLoader *ntpl.Loader
}

// DiscordMessage represents Discord message
type DiscordMessage struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents Discord embed
type DiscordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []DiscordField `json:"fields"`
	Footer      struct {
		Text    string `json:"text"`
		IconURL string `json:"icon_url,omitempty"`
	} `json:"footer"`
	Timestamp string `json:"timestamp"`
}

// DiscordField represents Discord field
type DiscordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// NewDiscordNotifier creates new Discord notifier
func NewDiscordNotifier(cfg *config.DiscordConfig, loader *ntpl.Loader, logger *zap.Logger) (*DiscordNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("discord notifier is disabled")
	}

	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("discord webhook URL is required")
	}

	return &DiscordNotifier{
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
func (d *DiscordNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	tmpl, err := d.tplLoader.GetTemplate(ntpl.Discord, "network_error")
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

	var msg DiscordMessage
	if err := json.Unmarshal(content.Bytes(), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	msg.Username = d.config.Username
	msg.AvatarURL = d.config.AvatarURL

	return d.send(msg)
}

// NotifyNetworkErrors sends network errors notification
func (d *DiscordNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	tmpl, err := d.tplLoader.GetTemplate(ntpl.Discord, "network_error")
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

	var msg DiscordMessage
	if err := json.Unmarshal(content.Bytes(), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	msg.Username = d.config.Username
	msg.AvatarURL = d.config.AvatarURL

	return d.send(msg)
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (d *DiscordNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	tmpl, err := d.tplLoader.GetTemplate(ntpl.Discord, "high_utilization")
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

	var msg DiscordMessage
	if err := json.Unmarshal(content.Bytes(), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	msg.Username = d.config.Username
	msg.AvatarURL = d.config.AvatarURL

	return d.send(msg)
}

// send sends Discord message
func (d *DiscordNotifier) send(msg DiscordMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	resp, err := d.client.Post(d.config.WebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		// Handle rate limiting
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
				time.Sleep(seconds)
				return d.send(msg) // Retry after waiting
			}
		}
		return fmt.Errorf("discord rate limit exceeded")
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discord api error: status code %d", resp.StatusCode)
	}

	return nil
}
