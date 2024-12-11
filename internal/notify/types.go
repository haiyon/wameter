package notify

import (
	"context"
	"wameter/internal/types"
)

// NotifierType represents the type of notifier
type NotifierType string

const (
	NotifierEmail    NotifierType = "email"
	NotifierTelegram NotifierType = "telegram"
	NotifierSlack    NotifierType = "slack"
	NotifierWeChat   NotifierType = "wechat"
	NotifierDingTalk NotifierType = "dingtalk"
	NotifierDiscord  NotifierType = "discord"
	NotifierWebhook  NotifierType = "webhook"
	NotifierFeishu   NotifierType = "feishu"
)

// Notifier represents notifier interface
type Notifier interface {
	// NotifyAgentOffline sends agent offline notification
	NotifyAgentOffline(agent *types.AgentInfo) error

	// NotifyNetworkErrors sends network errors notification
	NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error

	// NotifyHighNetworkUtilization sends high network utilization notification
	NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error

	// NotifyIPChange sends IP change notification
	NotifyIPChange(agent *types.AgentInfo, change *types.IPChange) error

	// Health checks the health of the notifier
	Health(ctx context.Context) error
}
