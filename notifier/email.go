package notifier

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"ip-monitor/config"
	"ip-monitor/types"
	"ip-monitor/utils"
)

// EmailNotifier handles email notifications
type EmailNotifier struct {
	config *config.Email
	tmpl   *template.Template
}

// emailData represents email template data
type emailData struct {
	Hostname     string
	Interface    string
	Changes      []string
	OldState     types.IPState
	NewState     types.IPState
	UpdatedAt    time.Time
	ShowIPv4     bool
	ShowIPv6     bool
	ShowExternal bool
	IsInitial    bool
}

const emailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; }
        .header { background: #f8f9fa; padding: 20px; }
        .content { padding: 20px; }
        .changes { background: #fff3cd; padding: 15px; margin: 10px 0; }
        .footer { color: #6c757d; font-size: 12px; padding: 20px; }
        .ip-group { margin: 10px 0; }
        .ip-group h4 { margin: 5px 0; }
        .ip-list { margin: 5px 0; padding-left: 20px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>{{if .IsInitial}}IP Monitor Initial State{{else}}IP Address Change Alert{{end}}</h2>
        {{if .IsInitial}}
        <p>IP Monitor has started. This is the current network configuration.</p>
        {{end}}
    </div>
    <div class="content">
        <p><strong>Host:</strong> {{.Hostname}}</p>
        <p><strong>Interface:</strong> {{.Interface}}</p>
        <p><strong>Time:</strong> {{.UpdatedAt.Format "2006-01-02 15:04:05"}}</p>

        {{if not .IsInitial}}
        <div class="changes">
            <h3>Changes Detected:</h3>
            <ul>
            {{range .Changes}}
                <li>{{.}}</li>
            {{end}}
            </ul>
        </div>
        {{end}}

        <h3>Current State:</h3>
        {{if .ShowIPv4}}
        <div class="ip-group">
            <h4>IPv4 Addresses:</h4>
            <ul class="ip-list">
            {{range .NewState.IPv4}}
                <li>{{.}}</li>
            {{end}}
            </ul>
        </div>
        {{end}}

        {{if .ShowIPv6}}
        <div class="ip-group">
            <h4>IPv6 Addresses:</h4>
            <ul class="ip-list">
            {{range .NewState.IPv6}}
                <li>{{.}}</li>
            {{end}}
            </ul>
        </div>
        {{end}}

        {{if and .ShowExternal .NewState.ExternalIP}}
        <div class="ip-group">
            <h4>External IP:</h4>
            <p>{{.NewState.ExternalIP}}</p>
        </div>
        {{end}}
    </div>
    <div class="footer">
        <p>This is an automated message. Please do not reply.</p>
        <p>Generated at: {{.UpdatedAt.Format "2006-01-02 15:04:05 MST"}}</p>
    </div>
</body>
</html>`

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(cfg *config.Email) (*EmailNotifier, error) {
	tmpl, err := template.New("email").Parse(emailTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email template: %w", err)
	}

	return &EmailNotifier{
		config: cfg,
		tmpl:   tmpl,
	}, nil
}

// Send sends an email notification about IP changes
func (n *EmailNotifier) Send(oldState, newState types.IPState, changes []string, iface string, opts notificationOptions) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Parse From address
	from, err := mail.ParseAddress(n.config.From)
	if err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}

	// Parse To addresses
	var toAddresses []string
	for _, addr := range n.config.To {
		to, err := mail.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("invalid to address '%s': %w", addr, err)
		}
		toAddresses = append(toAddresses, to.Address)
	}

	// Prepare email data
	data := emailData{
		Hostname:     hostname,
		Interface:    iface,
		Changes:      changes,
		OldState:     oldState,
		NewState:     newState,
		UpdatedAt:    time.Now(),
		ShowIPv4:     opts.showIPv4,
		ShowIPv6:     opts.showIPv6,
		ShowExternal: opts.showExternal,
		IsInitial:    opts.isInitial,
	}

	// Render email template
	var body bytes.Buffer
	if err := n.tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	subject := "IP Address Change Alert - %s"
	if opts.isInitial {
		subject = "IP Monitor Started - Initial State - %s"
	}

	// Prepare email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: "+subject+"\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n%s",
		from.String(),
		strings.Join(toAddresses, ", "),
		hostname,
		body.String())

	// Send email with retry
	err = utils.Retry(3, time.Second, func() error {
		return n.sendMail(from.Address, toAddresses, []byte(msg))
	})

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// sendMail handles the actual email sending process
func (n *EmailNotifier) sendMail(from string, to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)

	var conn net.Conn
	var err error
	var client *smtp.Client

	// Connect to SMTP server
	if n.config.UseTLS {
		// Create TLS connection
		tlsConfig := &tls.Config{
			ServerName: n.config.SMTPServer,
		}
		conn, err = tls.Dial("tcp", addr, tlsConfig)
	} else {
		conn, err = net.Dial("tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err = smtp.NewClient(conn, n.config.SMTPServer)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit() // Changed: just call Quit() without checking the error

	// Start TLS if not already using TLS and server requires it
	if !n.config.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			cfg := &tls.Config{ServerName: n.config.SMTPServer}
			if err = client.StartTLS(cfg); err != nil {
				return fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	// Authenticate if credentials are provided
	if n.config.Username != "" && n.config.Password != "" {
		auth := smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPServer)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", recipient, err)
		}
	}

	// Send message
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to create message writer: %w", err)
	}

	_, err = writer.Write(msg)
	if err != nil {
		writer.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close message writer: %w", err)
	}

	return nil // Success - don't check Quit() error as it's just cleanup
}
