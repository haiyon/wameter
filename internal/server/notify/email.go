package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"
	ntpl "wameter/internal/server/notify/template"

	"wameter/internal/server/config"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// EmailNotifier represents email notifier
type EmailNotifier struct {
	config    *config.EmailConfig
	logger    *zap.Logger
	tplLoader *ntpl.Loader
}

// TemplateData represents the data structure for email templates
type TemplateData struct {
	Subject    string
	Timestamp  time.Time
	Agent      *types.AgentInfo
	Interface  *types.InterfaceInfo
	AgentID    string
	FormatFunc template.FuncMap
}

// NewEmailNotifier creates new Email notifier
func NewEmailNotifier(cfg *config.EmailConfig, loader *ntpl.Loader, logger *zap.Logger) (*EmailNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("email notifier is disabled")
	}

	if cfg.SMTPServer == "" || cfg.From == "" || len(cfg.To) == 0 {
		return nil, fmt.Errorf("incomplete email configuration")
	}

	return &EmailNotifier{
		config:    cfg,
		logger:    logger,
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends agent offline notification
func (n *EmailNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	// Get template
	tmpl, err := n.tplLoader.GetTemplate(ntpl.Email, "agent_offline")
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

	// Send email
	subject := fmt.Sprintf("Agent Offline Alert - %s", agent.Hostname)
	return n.sendEmail(subject, content.String())
}

// NotifyNetworkErrors sends network errors notification
func (n *EmailNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	// Get template
	tmpl, err := n.tplLoader.GetTemplate(ntpl.Email, "network_error")
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

	subject := fmt.Sprintf("Network Errors Alert - %s - %s", agentID, iface.Name)
	return n.sendEmail(subject, content.String())
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (n *EmailNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	// Get template
	tmpl, err := n.tplLoader.GetTemplate(ntpl.Email, "high_utilization")
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

	subject := fmt.Sprintf("High Network Utilization - %s - %s", agentID, iface.Name)
	return n.sendEmail(subject, content.String())
}

// sendEmail sends an email
func (n *EmailNotifier) sendEmail(subject, content string) error {
	auth := smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPServer)

	msg := buildEmailMessage(n.config.From, n.config.To, subject, content)

	var err error
	if n.config.UseTLS {
		err = n.sendTLSEmail(auth, msg)
	} else {
		addr := fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)
		err = smtp.SendMail(addr, auth, n.config.From, n.config.To, msg)
	}

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// sendTLSEmail sends email
func (n *EmailNotifier) sendTLSEmail(auth smtp.Auth, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)

	tlsConfig := &tls.Config{
		ServerName: n.config.SMTPServer,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create TLS connection: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, n.config.SMTPServer)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err = client.Mail(n.config.From); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, addr := range n.config.To {
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT TO failed: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}
	defer w.Close()

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// buildEmailMessage builds email message
func buildEmailMessage(from string, to []string, subject, body string) []byte {
	var msg bytes.Buffer

	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ";")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.Bytes()
}
