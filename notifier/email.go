package notifier

import (
	"bytes"
	"fmt"
	"html/template"
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
	Hostname  string
	Interface string
	Changes   []string
	OldState  types.IPState
	NewState  types.IPState
	UpdatedAt time.Time
	IsInitial bool
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
        .initial-notice { background: #d4edda; padding: 15px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="header">
        <h2>{{if .IsInitial}}IP Monitor Initial State{{else}}IP Address Change Alert{{end}}</h2>
    </div>
    <div class="content">
        <p><strong>Host:</strong> {{.Hostname}}</p>
        <p><strong>Interface:</strong> {{.Interface}}</p>
        <p><strong>Time:</strong> {{.UpdatedAt.Format "2006-01-02 15:04:05"}}</p>

        {{if .IsInitial}}
        <div class="initial-notice">
            <h3>Initial State Notification</h3>
            <p>IP Monitor has started. This is the current network configuration.</p>
        </div>
        {{else}}
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
        <div class="ip-group">
            <h4>IPv4 Addresses:</h4>
            <ul class="ip-list">
            {{range .NewState.IPv4}}
                <li>{{.}}</li>
            {{end}}
            </ul>
        </div>

        <div class="ip-group">
            <h4>IPv6 Addresses:</h4>
            <ul class="ip-list">
            {{range .NewState.IPv6}}
                <li>{{.}}</li>
            {{end}}
            </ul>
        </div>

        {{if .NewState.ExternalIP}}
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
func NewEmailNotifier(config *config.Email) (*EmailNotifier, error) {
	tmpl, err := template.New("email").Parse(emailTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email template: %w", err)
	}

	return &EmailNotifier{
		config: config,
		tmpl:   tmpl,
	}, nil
}

// Send sends an email notification about IP changes
func (n *EmailNotifier) Send(oldState, newState types.IPState, changes []string, iface string, isInitial bool) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Prepare email data
	data := emailData{
		Hostname:  hostname,
		Interface: iface,
		Changes:   changes,
		OldState:  oldState,
		NewState:  newState,
		UpdatedAt: time.Now(),
		IsInitial: isInitial,
	}

	// Render email template
	var body bytes.Buffer
	if err := n.tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	subject := "IP Address Change Alert - %s"
	if isInitial {
		subject = "IP Monitor Started - Initial State - %s"
	}

	// Prepare email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: "+subject+"\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n%s",
		n.config.From,
		strings.Join(n.config.To, ","),
		hostname,
		body.String())

	// Configure TLS if enabled
	var auth smtp.Auth
	var addr string
	if n.config.UseTLS {
		auth = smtp.PlainAuth("",
			n.config.Username,
			n.config.Password,
			n.config.SMTPServer)
		addr = fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)
	} else {
		addr = fmt.Sprintf("%s:%d", n.config.SMTPServer, n.config.SMTPPort)
	}

	// Send email with retry
	err = utils.Retry(3, time.Second, func() error {
		return smtp.SendMail(addr, auth, n.config.From, n.config.To, []byte(msg))
	})

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
