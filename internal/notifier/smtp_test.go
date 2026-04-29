package notifier

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/maxlesscode/watchdog/internal/models"
)

func TestSMTPConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     SMTPConfig
		wantErr bool
	}{
		{
			name:    "all fields present",
			cfg:     SMTPConfig{Host: "smtp.example.com", Port: "587", User: "u", Pass: "p", AlertEmail: "a@b.com"},
			wantErr: false,
		},
		{
			name:    "all fields missing",
			cfg:     SMTPConfig{},
			wantErr: true,
		},
		{
			name:    "missing host",
			cfg:     SMTPConfig{Port: "587", User: "u", Pass: "p", AlertEmail: "a@b.com"},
			wantErr: true,
		},
		{
			name:    "missing alert email",
			cfg:     SMTPConfig{Host: "smtp.example.com", Port: "587", User: "u", Pass: "p"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSanitizeHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"clean string", "clean string"},
		// \r is stripped, \n becomes a space
		{"inject\r\nHeader: evil", "inject Header: evil"},
		{"newline\nonly", "newline only"},
		{"carriage\rreturn", "carriagereturn"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := sanitizeHeader(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNotify_ContextAlreadyCancelled(t *testing.T) {
	t.Parallel()

	// Start a listener that never accepts connections to confirm no dial happens.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	n := NewSMTPNotifier(SMTPConfig{
		Host:       host,
		Port:       port,
		User:       "u@example.com",
		Pass:       "secret",
		AlertEmail: "alert@example.com",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	err = n.Notify(ctx, models.Product{Name: "Widget", ActualPrice: 9.99, TargetPrice: 10.0})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
