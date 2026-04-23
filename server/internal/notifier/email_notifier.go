package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

type EmailNotifier struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewEmailNotifier(host string, port int, username, password, from string) *EmailNotifier {
	return &EmailNotifier{
		host:     strings.TrimSpace(host),
		port:     port,
		username: strings.TrimSpace(username),
		password: password,
		from:     strings.TrimSpace(from),
	}
}

func (n *EmailNotifier) Enabled() bool {
	return n != nil && n.host != "" && n.port > 0
}

// Notify sends a plain-text email to all recipients in the payload.
// Expected payload shape: map[string]any{
//   "to": []string{...},
//   "subject": string,
//   "body": string,
// }
func (n *EmailNotifier) Notify(ctx context.Context, payload any) error {
	if !n.Enabled() {
		return nil
	}

	p, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("email payload must be a map")
	}

	to, ok := p["to"].([]any)
	if !ok || len(to) == 0 {
		return fmt.Errorf("email payload missing 'to' recipients")
	}

	recipients := make([]string, 0, len(to))
	for _, r := range to {
		if s, ok := r.(string); ok && s != "" {
			recipients = append(recipients, s)
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("no valid email recipients")
	}

	subject, _ := p["subject"].(string)
	if subject == "" {
		subject = "DNS Hub Notification"
	}

	body, _ := p["body"].(string)
	if body == "" {
		body = "(no content)"
	}

	msg := buildEmailMessage(n.from, recipients, subject, body)
	return n.send(ctx, n.from, recipients, msg)
}

func buildEmailMessage(from string, to []string, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

func (n *EmailNotifier) send(ctx context.Context, from string, to []string, msg string) error {
	addr := fmt.Sprintf("%s:%d", n.host, n.port)

	var auth smtp.Auth
	if n.username != "" {
		auth = smtp.PlainAuth("", n.username, n.password, n.host)
	}

	// Run SMTP send in a goroutine so it respects context cancellation.
	errCh := make(chan error, 1)
	go func() {
		var sendErr error
		if n.port == 465 {
			sendErr = n.sendWithTLS(from, to, msg, auth)
		} else {
			sendErr = smtp.SendMail(addr, auth, from, to, []byte(msg))
		}
		errCh <- sendErr
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case sendErr := <-errCh:
		return sendErr
	}
}

func (n *EmailNotifier) sendWithTLS(from string, to []string, msg string, auth smtp.Auth) error {
	tlsConfig := &tls.Config{
		ServerName: n.host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", n.host, n.port), tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, n.host)
	if err != nil {
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	for _, r := range to {
		if err := client.Rcpt(r); err != nil {
			return fmt.Errorf("SMTP RCPT TO: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("SMTP write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP data close: %w", err)
	}
	return client.Quit()
}
