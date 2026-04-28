package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
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
type SMTPNotifier struct {
	cfg SMTPConfig
}

// NewSMTPNotifier returns an SMTPNotifier configured with cfg.
func NewSMTPNotifier(cfg SMTPConfig) *SMTPNotifier {
	return &SMTPNotifier{cfg: cfg}
}

// Notify sends a price-drop alert email for p.
// The TCP dial and all subsequent SMTP commands honor ctx — no goroutine is leaked.
func (n *SMTPNotifier) Notify(ctx context.Context, p models.Product) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	addr := n.cfg.Host + ":" + n.cfg.Port

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}

	// Propagate context deadline to all SMTP I/O so the connection doesn't
	// outlive cancellation waiting for a slow or non-responsive server.
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline) //nolint:errcheck
	}

	c, err := smtp.NewClient(conn, n.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer c.Close()

	if err := c.StartTLS(&tls.Config{ServerName: n.cfg.Host}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}

	if err := c.Auth(smtp.PlainAuth("", n.cfg.User, n.cfg.Pass, n.cfg.Host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err := c.Mail(n.cfg.User); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(n.cfg.AlertEmail); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	subject := fmt.Sprintf("Price alert: %s is now %.2f", sanitizeHeader(p.Name), p.ActualPrice)
	body := fmt.Sprintf(
		"Product:       %s\nURL:           %s\nCurrent price: %.2f\nTarget price:  %.2f",
		sanitizeHeader(p.Name), sanitizeHeader(p.URL), p.ActualPrice, p.TargetPrice,
	)
	msg := "To: " + n.cfg.AlertEmail + "\r\n" +
		"From: " + n.cfg.User + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body

	if _, err = fmt.Fprint(wc, msg); err != nil {
		wc.Close()
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp end data: %w", err)
	}

	return c.Quit()
}
