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

	"ip-monitor/config"
	"ip-monitor/types"
	"ip-monitor/utils"
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
func (n *TelegramNotifier) Send(oldState, newState types.IPState, changes []InterfaceChange, opts notificationOptions) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Prepare message text
	message := formatTelegramMessage(hostname, &oldState, &newState, changes, opts)

	messageLength := len(message)
	if messageLength > 4096 {
		// Split message if it's too long (Telegram has a 4096 character limit)
		messages := splitTelegramMessage(message)
		var errs []error

		// Send multiple messages
		for _, chatID := range n.config.ChatIDs {
			for i, msg := range messages {
				partMsg := msg
				if len(messages) > 1 {
					partMsg = fmt.Sprintf("(Part %d/%d)\n\n%s", i+1, len(messages), msg)
				}
				if err := n.sendMessage(chatID, partMsg); err != nil {
					errs = append(errs, fmt.Errorf("failed to send to chat %s: %w", chatID, err))
				}
			}
		}

		if len(errs) > 0 {
			return fmt.Errorf("multiple errors while sending telegram messages: %v", errs)
		}
	} else {
		// Send single message
		for _, chatID := range n.config.ChatIDs {
			if err := n.sendMessage(chatID, message); err != nil {
				return fmt.Errorf("failed to send to chat %s: %w", chatID, err)
			}
		}
	}

	return nil
}

// splitTelegramMessage splits a long message into parts that fit Telegram's limit
func splitTelegramMessage(message string) []string {
	const maxLength = 4000 // Leave some room for part numbers

	if len(message) <= maxLength {
		return []string{message}
	}

	var parts []string
	var currentPart strings.Builder
	lines := strings.Split(message, "\n")

	for _, line := range lines {
		// If adding this line would exceed the limit
		if currentPart.Len()+len(line)+1 > maxLength {
			// Save current part if it's not empty
			if currentPart.Len() > 0 {
				parts = append(parts, currentPart.String())
				currentPart.Reset()
			}
		}

		// Add the line to current part
		if currentPart.Len() > 0 {
			currentPart.WriteString("\n")
		}
		currentPart.WriteString(line)
	}

	// Add the last part if it's not empty
	if currentPart.Len() > 0 {
		parts = append(parts, currentPart.String())
	}

	return parts
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned non-200 status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("empty response from telegram API")
	}

	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("failed to parse response (body: %s): %w", string(body), err)
	}

	if !telegramResp.OK {
		if telegramResp.Description != "" {
			return fmt.Errorf("telegram API error: %s", telegramResp.Description)
		}
		return fmt.Errorf("telegram API returned error without description")
	}

	return nil
}

// formatTelegramMessage formats a message for Telegram
func formatTelegramMessage(hostname string, oldState, newState *types.IPState, changes []InterfaceChange, opts notificationOptions) string {
	var b strings.Builder

	if opts.isInitial {
		b.WriteString("*IP Monitor Started - Initial State*\n\n")
	} else {
		b.WriteString("*IP Address Change Alert*\n\n")
	}

	b.WriteString(fmt.Sprintf("*Host:* `%s`\n", hostname))
	b.WriteString(fmt.Sprintf("*Time:* `%s`\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// Group changes by interface
	for _, ifaceChange := range changes {
		b.WriteString(fmt.Sprintf("*Interface: %s (%s)*\n", ifaceChange.Name, ifaceChange.Type))

		if len(ifaceChange.Changes) > 0 {
			b.WriteString("\nChanges:\n")
			for _, change := range ifaceChange.Changes {
				b.WriteString(fmt.Sprintf("• %s\n", change))
			}
		}

		if ifaceState, ok := newState.InterfaceInfo[ifaceChange.Name]; ok {
			if opts.showIPv4 && len(ifaceState.IPv4) > 0 {
				b.WriteString("\nIPv4 Addresses:\n")
				for _, ip := range ifaceState.IPv4 {
					b.WriteString(fmt.Sprintf("• `%s`\n", ip))
				}
			}

			if opts.showIPv6 && len(ifaceState.IPv6) > 0 {
				b.WriteString("\nIPv6 Addresses:\n")
				for _, ip := range ifaceState.IPv6 {
					b.WriteString(fmt.Sprintf("• `%s`\n", ip))
				}
			}
		}

		if ifaceChange.Stats != nil {
			b.WriteString("\nStatistics:\n")
			if ifaceChange.Stats.RxBytesRate > 0 {
				b.WriteString(fmt.Sprintf("• Rx Rate: `%s/s`\n",
					utils.FormatBytesRate(ifaceChange.Stats.RxBytesRate)))
			}
			if ifaceChange.Stats.TxBytesRate > 0 {
				b.WriteString(fmt.Sprintf("• Tx Rate: `%s/s`\n",
					utils.FormatBytesRate(ifaceChange.Stats.TxBytesRate)))
			}
		}

		b.WriteString("\n")
	}

	if opts.showExternal && newState.ExternalIP != "" {
		b.WriteString(fmt.Sprintf("\n*External IP:* `%s`\n", newState.ExternalIP))
	}

	return b.String()
}
