package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"wameter/internal/config"
	ntpl "wameter/internal/notify/template"
	"wameter/internal/types"
	"wameter/internal/utils"

	"go.uber.org/zap"
)

// TelegramNotifier represents Telegram notifier
type TelegramNotifier struct {
	config    *config.TelegramConfig
	logger    *zap.Logger
	client    *http.Client
	tplLoader *ntpl.Loader
}

// TelegramMessage represents Telegram message
type TelegramMessage struct {
	ChatID              string `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// NewTelegramNotifier creates new Telegram notifier
func NewTelegramNotifier(cfg *config.TelegramConfig, loader *ntpl.Loader, logger *zap.Logger) (*TelegramNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("telegram notifier is disabled")
	}

	if cfg.BotToken == "" || len(cfg.ChatIDs) == 0 {
		return nil, fmt.Errorf("telegram bot token and chat IDs are required")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 5,
		},
	}

	return &TelegramNotifier{
		config:    cfg,
		logger:    logger,
		client:    client,
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends agent offline notification
func (n *TelegramNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	message := fmt.Sprintf(
		"🚨 *Agent Offline Alert*\n\n"+
			"Agent has gone offline and requires attention.\n\n"+
			"*Details:*\n"+
			"• Agent ID: `%s`\n"+
			"• Hostname: `%s`\n"+
			"• Last Seen: `%s`\n"+
			"• Status: `%s`\n\n"+
			"_%s_",
		agent.ID,
		agent.Hostname,
		agent.LastSeen.Format(time.RFC3339),
		agent.Status,
		fmt.Sprintf("Alert generated at %s", time.Now().Format("2006-01-02 15:04:05")))

	return n.sendToAll(message)
}

// NotifyNetworkErrors sends network errors notification
func (n *TelegramNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	message := fmt.Sprintf(
		"⚠️ *Network Errors Alert*\n\n"+
			"High number of network errors detected.\n\n"+
			"*Interface Details:*\n"+
			"• Agent ID: `%s`\n"+
			"• Interface: `%s`\n"+
			"• Type: `%s`\n\n"+
			"*Error Statistics:*\n"+
			"• RX Errors: `%d`\n"+
			"• TX Errors: `%d`\n"+
			"• RX Dropped: `%d`\n"+
			"• TX Dropped: `%d`\n\n"+
			"_%s_",
		agentID,
		iface.Name,
		iface.Type,
		iface.Statistics.RxErrors,
		iface.Statistics.TxErrors,
		iface.Statistics.RxDropped,
		iface.Statistics.TxDropped,
		fmt.Sprintf("Alert generated at %s", time.Now().Format("2006-01-02 15:04:05")))

	return n.sendToAll(message)
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (n *TelegramNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	message := fmt.Sprintf(
		"📈 *High Network Utilization*\n\n"+
			"*Interface Details:*\n"+
			"• Agent ID: `%s`\n"+
			"• Interface: `%s`\n"+
			"• Type: `%s`\n\n"+
			"*Current Rates:*\n"+
			"• Receive: `%s/s`\n"+
			"• Transmit: `%s/s`\n\n"+
			"*Total Traffic:*\n"+
			"• Received: `%s`\n"+
			"• Transmitted: `%s`\n\n"+
			"_%s_",
		agentID,
		iface.Name,
		iface.Type,
		utils.FormatBytesRate(iface.Statistics.RxBytesRate),
		utils.FormatBytesRate(iface.Statistics.TxBytesRate),
		utils.FormatBytes(iface.Statistics.RxBytes),
		utils.FormatBytes(iface.Statistics.TxBytes),
		fmt.Sprintf("Alert generated at %s", time.Now().Format("2006-01-02 15:04:05")))

	return n.sendToAll(message)
}

// NotifyIPChange sends IP change notification
func (n *TelegramNotifier) NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error {
	var description string
	if change.IsExternal {
		description = fmt.Sprintf(
			"🌐 *IP Change Detected*\n\n"+
				"*External IP Change*\n"+
				"• Agent ID: `%s`\n"+
				"• Hostname: `%s`\n"+
				"• IP Version: `%s`\n"+
				"• Old IP: `%s`\n"+
				"• New IP: `%s`\n\n"+
				"_%s_",
			agent.ID,
			agent.Hostname,
			change.Version,
			strings.Join(change.OldAddrs, ", "),
			strings.Join(change.NewAddrs, ", "),
			fmt.Sprintf("Changed at %s", change.Timestamp.Format("2006-01-02 15:04:05")))
	} else {
		description = fmt.Sprintf(
			"🌐 *IP Change Detected*\n\n"+
				"*Interface IP Change*\n"+
				"• Agent ID: `%s`\n"+
				"• Hostname: `%s`\n"+
				"• Interface: `%s`\n"+
				"• IP Version: `%s`\n"+
				"• Old IPs: `%s`\n"+
				"• New IPs: `%s`\n\n"+
				"_%s_",
			agent.ID,
			agent.Hostname,
			change.InterfaceName,
			change.Version,
			strings.Join(change.OldAddrs, ", "),
			strings.Join(change.NewAddrs, ", "),
			fmt.Sprintf("Changed at %s", change.Timestamp.Format("2006-01-02 15:04:05")))
	}

	return n.sendToAll(description)
}

// sendToAll sends message to all chat IDs
func (n *TelegramNotifier) sendToAll(text string) error {
	var errors []string

	// Use proper format based on config
	format := strings.ToLower(n.config.Format)
	if format == "" {
		format = "markdown" // default format
	}

	for _, chatID := range n.config.ChatIDs {
		if err := n.sendMessage(chatID, text, format); err != nil {
			errors = append(errors, fmt.Sprintf("chat_id %s: %v", chatID, err))
			n.logger.Error("Failed to send telegram message",
				zap.Error(err),
				zap.String("chat_id", chatID))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send messages: %s", strings.Join(errors, "; "))
	}

	return nil
}

// sendMessage sends a message to a specific chat ID
func (n *TelegramNotifier) sendMessage(chatID, text, format string) error {
	msg := TelegramMessage{
		ChatID:    chatID,
		Text:      text,
		ParseMode: format,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.config.BotToken)

	// Create request with context and timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
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

	if resp.StatusCode == http.StatusTooManyRequests {
		// Handle rate limiting
		var rateLimitResp struct {
			Parameters struct {
				RetryAfter int `json:"retry_after"`
			} `json:"parameters"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rateLimitResp); err == nil {
			time.Sleep(time.Duration(rateLimitResp.Parameters.RetryAfter) * time.Second)
			return n.sendMessage(chatID, text, format) // Retry after waiting
		}
		return fmt.Errorf("rate limit exceeded")
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Description string `json:"description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return fmt.Errorf("telegram API error: status %d", resp.StatusCode)
		}
		return fmt.Errorf("telegram API error: %s", errorResp.Description)
	}

	return nil
}

// Health checks the health of the notifier
func (n *TelegramNotifier) Health(_ context.Context) error {
	// Note: Add health check logic here
	return nil
}
