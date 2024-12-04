package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
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

// NewEmailNotifier creates new Email notifier
func NewEmailNotifier(cfg *config.EmailConfig, loader *ntpl.Loader, logger *zap.Logger) (*EmailNotifier, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("email notifier is disabled")
	}

	return &EmailNotifier{
		config:    cfg,
		logger:    logger,
		tplLoader: loader,
	}, nil
}

// NotifyAgentOffline sends agent offline notification
func (n *EmailNotifier) NotifyAgentOffline(agent *types.AgentInfo) error {
	data := map[string]any{
		"Agent":     agent,
		"Timestamp": time.Now(),
	}
	subject := fmt.Sprintf("Agent Offline Alert - %s", agent.Hostname)
	return n.sendTemplateEmail("agent_offline", data, subject)
}

// NotifyNetworkErrors sends network errors notification
func (n *EmailNotifier) NotifyNetworkErrors(agentID string, iface *types.InterfaceInfo) error {
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}
	subject := fmt.Sprintf("Network Errors Alert - %s - %s", agentID, iface.Name)
	return n.sendTemplateEmail("network_error", data, subject)
}

// NotifyHighNetworkUtilization sends high network utilization notification
func (n *EmailNotifier) NotifyHighNetworkUtilization(agentID string, iface *types.InterfaceInfo) error {
	data := map[string]any{
		"AgentID":   agentID,
		"Interface": iface,
		"Timestamp": time.Now(),
	}
	subject := fmt.Sprintf("High Network Utilization - %s - %s", agentID, iface.Name)
	return n.sendTemplateEmail("high_utilization", data, subject)
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

// sendTLSEmail sends email with explicit connection handling
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

	// Validate and clean the from address
	from := n.config.From
	if !strings.Contains(from, "@") {
		return fmt.Errorf("invalid from address: %s", from)
	}
	from = cleanEmailAddress(from)

	// Set sender
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed for %s: %w", from, err)
	}

	// Add recipients
	cleanTo := cleanEmailAddresses(n.config.To)
	for _, addr := range cleanTo {
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT TO failed for %s: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	if _, err = w.Write(msg); err != nil {
		w.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close message writer: %w", err)
	}
	return client.Quit()
}

// sendTemplateEmail sends an email
func (n *EmailNotifier) sendTemplateEmail(templateName string, data map[string]any, subject string) error {
	tmpl, err := n.tplLoader.GetTemplate(ntpl.Email, templateName)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return n.sendEmail(subject, content.String())
}

// buildEmailMessage builds email message
func buildEmailMessage(from string, to []string, subject, body string) []byte {
	var msg bytes.Buffer

	// Clean and format addresses
	cleanFrom := cleanEmailAddress(from)
	cleanTo := cleanEmailAddresses(to)

	// Add headers with proper line endings
	headers := map[string]string{
		"From":         cleanFrom,
		"To":           strings.Join(cleanTo, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
		"X-Mailer":     "Wameter/1.0",
		"Date":         time.Now().Format(time.RFC1123Z),
	}

	for key, value := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	msg.WriteString("\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n")

	return msg.Bytes()
}

// cleanEmailAddress cleans email address by removing display name and angle brackets
func cleanEmailAddress(addr string) string {
	if idx := strings.LastIndex(addr, "<"); idx >= 0 {
		return strings.Trim(addr[idx:], "<>")
	}
	return addr
}

// cleanEmailAddresses cleans a list of email addresses
func cleanEmailAddresses(addrs []string) []string {
	cleaned := make([]string, len(addrs))
	for i, addr := range addrs {
		cleaned[i] = cleanEmailAddress(addr)
	}
	return cleaned
}
