// server/notify/telegram.go
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/haiyon/wameter/internal/server/config"
	"github.com/haiyon/wameter/internal/types"
	"github.com/haiyon/wameter/internal/utils"
	"go.uber.org/zap"
)

type TelegramNotifier struct {
	config *config.TelegramConfig
	logger *zap.Logger
	client *http.Client
}

type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func NewTelegramNotifier(cfg *config.TelegramConfig, logger *zap.Logger) (*TelegramNotifier, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
		},
	}

	return &TelegramNotifier{
		config: cfg,
		logger: logger,
		client: client,
	}, nil
}

func (n *TelegramNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	message := fmt.Sprintf(
		"🔴 *Agent Offline Alert*\n\n"+
			"*Agent ID:* `%s`\n"+
			"*Hostname:* `%s`\n"+
			"*Last Seen:* `%s`\n"+
			"*Status:* `%s`\n\n"+
			"_Alert generated at %s_",
		agent.ID,
		agent.Hostname,
		agent.LastSeen.Format("2006-01-02 15:04:05"),
		agent.Status,
		time.Now().Format("2006-01-02 15:04:05"))

	return n.sendToAll(message)
}

func (n *TelegramNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	message := fmt.Sprintf(
		"⚠️ *Network Errors Alert*\n\n"+
			"*Agent ID:* `%s`\n"+
			"*Interface:* `%s` (%s)\n\n"+
			"*Statistics:*\n"+
			"• Rx Errors: `%d`\n"+
			"• Tx Errors: `%d`\n"+
			"• Dropped Packets (rx/tx): `%d/%d`\n\n"+
			"_Alert generated at %s_",
		agentID,
		iface.Name,
		iface.Type,
		iface.Statistics.RxErrors,
		iface.Statistics.TxErrors,
		iface.Statistics.RxDropped,
		iface.Statistics.TxDropped,
		time.Now().Format("2006-01-02 15:04:05"))

	return n.sendToAll(message)
}

func (n *TelegramNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	message := fmt.Sprintf(
		"📈 *High Network Utilization*\n\n"+
			"*Agent ID:* `%s`\n"+
			"*Interface:* `%s` (%s)\n\n"+
			"*Current Rates:*\n"+
			"• Receive: `%s/s`\n"+
			"• Transmit: `%s/s`\n\n"+
			"*Total Traffic:*\n"+
			"• Received: `%s`\n"+
			"• Transmitted: `%s`\n\n"+
			"_Alert generated at %s_",
		agentID,
		iface.Name,
		iface.Type,
		utils.FormatBytesRate(iface.Statistics.RxBytesRate),
		utils.FormatBytesRate(iface.Statistics.TxBytesRate),
		utils.FormatBytes(iface.Statistics.RxBytes),
		utils.FormatBytes(iface.Statistics.TxBytes),
		time.Now().Format("2006-01-02 15:04:05"))

	return n.sendToAll(message)
}

func (n *TelegramNotifier) sendToAll(text string) error {
	var errors []string

	for _, chatID := range n.config.ChatIDs {
		if err := n.sendMessage(chatID, text); err != nil {
			errors = append(errors, fmt.Sprintf("chat_id %s: %v", chatID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send messages: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (n *TelegramNotifier) sendMessage(chatID string, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.config.BotToken)

	msg := telegramMessage{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	resp, err := n.client.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

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
