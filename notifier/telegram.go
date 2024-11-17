package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/haiyon/ip-monitor/config"
	"github.com/haiyon/ip-monitor/types"
	"github.com/haiyon/ip-monitor/utils"
)

// TelegramNotifier handles Telegram notifications
type TelegramNotifier struct {
	config *config.Telegram
	client *http.Client
}

// TelegramMessage represents a Telegram API message
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// TelegramResponse represents a Telegram API response
type TelegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// NewTelegramNotifier creates a new Telegram notifier
func NewTelegramNotifier(config *config.Telegram) (*TelegramNotifier, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	return &TelegramNotifier{
		config: config,
		client: client,
	}, nil
}

// Send sends a Telegram notification about IP changes
func (n *TelegramNotifier) Send(oldState, newState types.IPState, changes []string) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Prepare message text
	message := formatTelegramMessage(hostname, &oldState, &newState, changes)

	if newState.ExternalIP != "" {
		message += fmt.Sprintf("External IP: `%s`\n", newState.ExternalIP)
	}

	// Send to all configured chat IDs
	var lastErr error
	for _, chatID := range n.config.ChatIDs {
		if err := n.sendMessage(chatID, message); err != nil {
			lastErr = err
			continue
		}
	}

	if lastErr != nil {
		return fmt.Errorf("failed to send telegram messages: %w", lastErr)
	}

	return nil
}

// sendMessage sends a message to a specific chat ID
func (n *TelegramNotifier) sendMessage(chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage",
		n.config.BotToken)

	msg := TelegramMessage{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	// Retry the send operation
	return utils.Retry(3, time.Second, func() error {
		return n.doSendMessage(url, msg)
	})
}

// formatTelegramMessage formats a message for Telegram
func formatTelegramMessage(hostname string, oldState, newState *types.IPState, changes []string) string {
	var b strings.Builder

	b.WriteString("*IP Address Change Alert*\n\n")
	b.WriteString(fmt.Sprintf("*Host:* `%s`\n", hostname))
	b.WriteString(fmt.Sprintf("*Time:* `%s`\n\n", time.Now().Format("2006-01-02 15:04:05")))

	b.WriteString("*Changes:*\n")
	for _, change := range changes {
		b.WriteString(fmt.Sprintf("• %s\n", change))
	}
	b.WriteString("\n")

	b.WriteString("*Current State:*\n")
	if len(newState.IPv4) > 0 {
		b.WriteString("\nIPv4 Addresses:\n")
		for _, ip := range newState.IPv4 {
			b.WriteString(fmt.Sprintf("• `%s`\n", ip))
		}
	}

	if len(newState.IPv6) > 0 {
		b.WriteString("\nIPv6 Addresses:\n")
		for _, ip := range newState.IPv6 {
			b.WriteString(fmt.Sprintf("• `%s`\n", ip))
		}
	}

	if newState.ExternalIP != "" {
		b.WriteString(fmt.Sprintf("\nExternal IP: `%s`\n", newState.ExternalIP))
	}

	return b.String()
}

// doSendMessage performs the actual message sending
func (n *TelegramNotifier) doSendMessage(url string, msg TelegramMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API error: %s", telegramResp.Description)
	}

	return nil
}
