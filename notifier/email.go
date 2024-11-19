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
	Changes      []InterfaceChange
	OldState     types.IPState
	NewState     types.IPState
	UpdatedAt    time.Time
	ShowIPv4     bool
	ShowIPv6     bool
	ShowExternal bool
	IsInitial    bool
}

// InterfaceChange represents changes for a specific interface
type InterfaceChange struct {
	Name    string
	Type    string
	Changes []string
	Stats   *types.InterfaceStats
	Status  string
}

const emailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .header { background: #f8f9fa; padding: 20px; border-radius: 5px; }
        .content { padding: 20px; }
        .changes { background: #fff3cd; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .footer { color: #6c757d; font-size: 12px; padding: 20px; border-top: 1px solid #dee2e6; }
        .ip-group { margin: 10px 0; }
        .ip-group h4 { margin: 5px 0; color: #495057; }
        .ip-list { margin: 5px 0; padding-left: 20px; }
        .interface-section {
            background: #ffffff;
            border: 1px solid #dee2e6;
            border-radius: 5px;
            margin: 15px 0;
            padding: 15px;
        }
        .interface-header {
            display: flex;
            justify-content: space-between;
            border-bottom: 1px solid #dee2e6;
            padding-bottom: 10px;
            margin-bottom: 10px;
        }
        .status {
            padding: 3px 8px;
            border-radius: 3px;
            font-size: 12px;
        }
        .status-up { background: #d4edda; color: #155724; }
        .status-down { background: #f8d7da; color: #721c24; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 10px;
            margin-top: 10px;
        }
        .stat-item {
            background: #f8f9fa;
            padding: 8px;
            border-radius: 3px;
        }
        .stat-label { font-size: 12px; color: #6c757d; }
        .stat-value { font-size: 14px; font-weight: bold; }
    </style>
</head>
<body>
    <div class="header">
        <h2>{{if .IsInitial}}IP Monitor Initial State{{else}}IP Address Change Alert{{end}}</h2>
        <p><strong>Host:</strong> {{.Hostname}}</p>
        <p><strong>Time:</strong> {{.UpdatedAt.Format "2006-01-02 15:04:05"}}</p>
    </div>

    <div class="content">
        {{range .Changes}}
        <div class="interface-section">
            <div class="interface-header">
                <h3>Interface: {{.Name}} ({{.Type}})</h3>
                <span class="status {{if eq .Status "up"}}status-up{{else}}status-down{{end}}">
                    {{.Status}}
                </span>
            </div>

            {{if .Changes}}
            <div class="changes">
                <h4>Changes Detected:</h4>
                <ul>
                {{range .Changes}}
                    <li>{{.}}</li>
                {{end}}
                </ul>
            </div>
            {{end}}

            {{if .Stats}}
            <div class="stats-grid">
                {{if gti .Stats.RxBytes 0}}
                <div class="stat-item">
                    <div class="stat-label">Received</div>
                    <div class="stat-value">{{formatBytes .Stats.RxBytes}}</div>
                </div>
                {{end}}
                {{if gti .Stats.TxBytes 0}}
                <div class="stat-item">
                    <div class="stat-label">Transmitted</div>
                    <div class="stat-value">{{formatBytes .Stats.TxBytes}}</div>
                </div>
                {{end}}
                {{if gtf .Stats.RxBytesRate 0}}
                <div class="stat-item">
                    <div class="stat-label">Receive Rate</div>
                    <div class="stat-value">{{formatBytesRate .Stats.RxBytesRate}}/s</div>
                </div>
                {{end}}
                {{if gtf .Stats.TxBytesRate 0}}
                <div class="stat-item">
                    <div class="stat-label">Transmit Rate</div>
                    <div class="stat-value">{{formatBytesRate .Stats.TxBytesRate}}/s</div>
                </div>
                {{end}}
            </div>
            {{end}}

            {{with $state := index $.NewState.InterfaceInfo .Name}}
                {{if and $.ShowIPv4 $state.IPv4}}
                <div class="ip-group">
                    <h4>IPv4 Addresses:</h4>
                    <ul class="ip-list">
                    {{range $state.IPv4}}
                        <li>{{.}}</li>
                    {{end}}
                    </ul>
                </div>
                {{end}}

                {{if and $.ShowIPv6 $state.IPv6}}
                <div class="ip-group">
                    <h4>IPv6 Addresses:</h4>
                    <ul class="ip-list">
                    {{range $state.IPv6}}
                        <li>{{.}}</li>
                    {{end}}
                    </ul>
                </div>
                {{end}}
            {{end}}
        </div>
        {{end}}

        {{if and .ShowExternal .NewState.ExternalIP}}
        <div class="interface-section">
            <h3>External IP</h3>
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
	funcMap := template.FuncMap{
		"formatBytes":     utils.FormatBytes,
		"formatBytesRate": utils.FormatBytesRate,
		"gtf": func(a, b float64) bool {
			return a > b
		},
		"gti": func(a, b uint64) bool {
			return a > b
		},
	}
	tmpl, err := template.New("email").Funcs(funcMap).Parse(emailTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email template: %w", err)
	}

	return &EmailNotifier{
		config: cfg,
		tmpl:   tmpl,
	}, nil
}

// Send sends an email notification about IP changes
func (n *EmailNotifier) Send(oldState, newState types.IPState, changes []InterfaceChange, opts notificationOptions) error {
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

	// Prepare subject based on changes
	subject := n.formatEmailSubject(hostname, changes, opts.isInitial)

	// Prepare email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n%s",
		from.String(),
		strings.Join(toAddresses, ", "),
		subject,
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

// formatEmailSubject generates an appropriate email subject based on the changes
func (n *EmailNotifier) formatEmailSubject(hostname string, changes []InterfaceChange, isInitial bool) string {
	if isInitial {
		return fmt.Sprintf("IP Monitor Started - Initial State - %s", hostname)
	}

	// Count interfaces with changes
	changedIfaces := 0
	hasExternalChange := false
	for _, change := range changes {
		if change.Name == "External" {
			hasExternalChange = true
		} else {
			changedIfaces++
		}
	}

	if changedIfaces == 0 && hasExternalChange {
		return fmt.Sprintf("External IP Change - %s", hostname)
	} else if changedIfaces == 1 {
		// Get the name of the changed interface
		var ifaceName string
		for _, change := range changes {
			if change.Name != "External" {
				ifaceName = change.Name
				break
			}
		}
		return fmt.Sprintf("IP Change on %s - %s", ifaceName, hostname)
	}

	return fmt.Sprintf("IP Changes on %d Interfaces - %s", changedIfaces, hostname)
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
