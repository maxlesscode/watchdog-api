package notifier

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/maxlesscode/watchdog/internal/models"
)

// sanitizeHeader strips CR and LF to prevent email header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", " ")
}

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host       string
	Port       string
	User       string
	Pass       string
	AlertEmail string
}

// Validate returns an error listing any missing required fields.
func (c SMTPConfig) Validate() error {
	var missing []string
	if c.Host == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if c.Port == "" {
		missing = append(missing, "SMTP_PORT")
	}
	if c.User == "" {
		missing = append(missing, "SMTP_USER")
	}
	if c.Pass == "" {
		missing = append(missing, "SMTP_PASS")
	}
	if c.AlertEmail == "" {
		missing = append(missing, "ALERT_EMAIL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing SMTP config: %s", strings.Join(missing, ", "))
	}
	return nil
}

// SMTPNotifier sends price-alert emails via SMTP STARTTLS (port 587).
// Port 465 (implicit TLS) is not supported in this implementation.
type SMTPNotifier struct {
	cfg SMTPConfig
}

// NewSMTPNotifier returns an SMTPNotifier configured with cfg.
func NewSMTPNotifier(cfg SMTPConfig) *SMTPNotifier {
	return &SMTPNotifier{cfg: cfg}
}

// Notify sends a price-drop alert email for p.
func (n *SMTPNotifier) Notify(ctx context.Context, p models.Product) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	addr := n.cfg.Host + ":" + n.cfg.Port
	auth := smtp.PlainAuth("", n.cfg.User, n.cfg.Pass, n.cfg.Host)

	subject := fmt.Sprintf("Price alert: %s is now %.2f", sanitizeHeader(p.Name), p.ActualPrice)
	body := fmt.Sprintf(
		"Product:       %s\nURL:           %s\nCurrent price: %.2f\nTarget price:  %.2f",
		sanitizeHeader(p.Name), sanitizeHeader(p.URL), p.ActualPrice, p.TargetPrice,
	)
	msg := []byte(
		"To: " + n.cfg.AlertEmail + "\r\n" +
			"From: " + n.cfg.User + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"\r\n" +
			body,
	)

	// smtp.SendMail blocks with no ctx support; race it against ctx cancellation.
	// The spawned goroutine may outlive cancellation until the OS TCP timeout fires.
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(addr, auth, n.cfg.User, []string{n.cfg.AlertEmail}, msg) }()

	select {
	case <-ctx.Done():
		return fmt.Errorf("smtp notify cancelled: %w", ctx.Err())
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send to %s: %w", n.cfg.AlertEmail, err)
		}
		return nil
	}
}
