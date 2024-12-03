package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"wameter/internal/server/config"
	"wameter/internal/types"
	"wameter/internal/utils"

	"go.uber.org/zap"
)

// EmailNotifier represents an email notifier
type EmailNotifier struct {
	config *config.EmailConfig
	logger *zap.Logger
	tmpl   *template.Template
}

// NewEmailNotifier creates new email notifier
func NewEmailNotifier(cfg *config.EmailConfig, logger *zap.Logger) (*EmailNotifier, error) {
	// Parse email templates
	tmpl, err := template.New("email").Funcs(template.FuncMap{
		"formatBytes":     utils.FormatBytes,
		"formatBytesRate": utils.FormatBytesRate,
		"formatTime":      formatTime,
	}).Parse(emailTemplates)

	if err != nil {
		return nil, fmt.Errorf("failed to parse email templates: %w", err)
	}

	return &EmailNotifier{
		config: cfg,
		logger: logger,
		tmpl:   tmpl,
	}, nil
}

// NotifyAgentOffline sends an email notification
func (n *EmailNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	data := struct {
		Agent     *types.AgentInfo
		TimeStamp time.Time
	}{
		Agent:     agent,
		TimeStamp: time.Now(),
	}

	subject := fmt.Sprintf("Agent Offline Alert - %s", agent.Hostname)
	return n.sendEmail(subject, "agent_offline", data)
}

// NotifyNetworkErrors sends an email notification
func (n *EmailNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	data := struct {
		AgentID   string
		Interface *types.InterfaceInfo
		TimeStamp time.Time
	}{
		AgentID:   agentID,
		Interface: iface,
		TimeStamp: time.Now(),
	}

	subject := fmt.Sprintf("Network Errors Alert - %s - %s", agentID, iface.Name)
	return n.sendEmail(subject, "network_errors", data)
}

// NotifyHighNetworkUtilization sends an email notification
func (n *EmailNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	data := struct {
		AgentID   string
		Interface *types.InterfaceInfo
		TimeStamp time.Time
	}{
		AgentID:   agentID,
		Interface: iface,
		TimeStamp: time.Now(),
	}

	subject := fmt.Sprintf("High Network Utilization - %s - %s", agentID, iface.Name)
	return n.sendEmail(subject, "high_utilization", data)
}

// sendEmail sends an email
func (n *EmailNotifier) sendEmail(subject, templateName string, data any) error {
	var body bytes.Buffer
	if err := n.tmpl.ExecuteTemplate(&body, templateName, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Prepare email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n%s", n.config.From,
		n.config.To[0], subject, body.String())

	// Configure TLS
	tlsConfig := &tls.Config{
		ServerName: n.config.SMTPServer,
		MinVersion: tls.VersionTLS12,
	}

	// Connect to SMTP server
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort), tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, n.config.SMTPServer)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	if n.config.Username != "" {
		auth := smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPServer)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender and recipients
	if err := client.Mail(n.config.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, to := range n.config.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", to, err)
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to create message writer: %w", err)
	}

	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close message writer: %w", err)
	}

	return nil
}

// formatTime formats a time.Time as a string with the format "2006-01-02 15:04:05"
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// Email templates
const emailTemplates = `
{{define "agent_offline"}}
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .alert { background: #ffe0e0; padding: 15px; border-radius: 5px; }
        .details { margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="alert">
            <h2>Agent Offline Alert</h2>
            <p>An agent has gone offline:</p>
        </div>
        <div class="details">
            <p><strong>Agent ID:</strong> {{.Agent.ID}}</p>
            <p><strong>Hostname:</strong> {{.Agent.Hostname}}</p>
            <p><strong>Last Seen:</strong> {{formatTime .Agent.LastSeen}}</p>
            <p><strong>Status:</strong> {{.Agent.Status}}</p>
        </div>
        <p><small>Alert generated at {{formatTime .TimeStamp}}</small></p>
    </div>
</body>
</html>
{{end}}

{{define "network_errors"}}
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .alert { background: #fff0e0; padding: 15px; border-radius: 5px; }
        .details { margin-top: 20px; }
        .stats { background: #f5f5f5; padding: 10px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="alert">
            <h2>Network Errors Detected</h2>
            <p>High number of network errors detected on interface:</p>
        </div>
        <div class="details">
            <p><strong>Agent ID:</strong> {{.AgentID}}</p>
            <p><strong>Interface:</strong> {{.Interface.Name}} ({{.Interface.Type}})</p>
            <div class="stats">
                <p><strong>Rx Errors:</strong> {{.Interface.Statistics.RxErrors}}</p>
                <p><strong>Tx Errors:</strong> {{.Interface.Statistics.TxErrors}}</p>
                <p><strong>Dropped Packets:</strong> {{.Interface.Statistics.RxDropped}} (rx) / {{.Interface.Statistics.TxDropped}} (tx)</p>
            </div>
        </div>
        <p><small>Alert generated at {{formatTime .TimeStamp}}</small></p>
    </div>
</body>
</html>
{{end}}

{{define "high_utilization"}}
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .alert { background: #e0f0ff; padding: 15px; border-radius: 5px; }
        .details { margin-top: 20px; }
        .stats { background: #f5f5f5; padding: 10px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="alert">
            <h2>High Network Utilization</h2>
            <p>High network utilization detected on interface:</p>
        </div>
        <div class="details">
            <p><strong>Agent ID:</strong> {{.AgentID}}</p>
            <p><strong>Interface:</strong> {{.Interface.Name}} ({{.Interface.Type}})</p>
            <div class="stats">
                <p><strong>Receive Rate:</strong> {{formatBytesRate .Interface.Statistics.RxBytesRate}}/s</p>
                <p><strong>Transmit Rate:</strong> {{formatBytesRate .Interface.Statistics.TxBytesRate}}/s</p>
                <p><strong>Total Received:</strong> {{formatBytes .Interface.Statistics.RxBytes}}</p>
                <p><strong>Total Transmitted:</strong> {{formatBytes .Interface.Statistics.TxBytes}}</p>
            </div>
        </div>
        <p><small>Alert generated at {{formatTime .TimeStamp}}</small></p>
    </div>
</body>
</html>
{{end}}
`
