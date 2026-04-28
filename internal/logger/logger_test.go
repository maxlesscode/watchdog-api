package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestNew_CreatesLogFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.log")
	l, cleanup, err := New(path, slog.LevelInfo, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cleanup)

	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func TestNew_CleanupClosesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.log")
	_, cleanup, err := New(path, slog.LevelInfo, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// calling cleanup twice must not panic
	cleanup()
	cleanup()
}

func TestNew_DevModeWritesToFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dev.log")
	l, cleanup, err := New(path, slog.LevelInfo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cleanup)

	l.Info("hello from dev mode")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("log file is empty after writing a message in dev mode")
	}
}

func TestNew_NonDevModeWritesToFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prod.log")
	l, cleanup, err := New(path, slog.LevelInfo, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cleanup)

	l.Info("hello from prod mode")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("log file is empty after writing a message in prod mode")
	}
}

func TestNew_UnwritablePath_ReturnsError(t *testing.T) {
	t.Parallel()

	_, cleanup, err := New("/nonexistent/dir/test.log", slog.LevelInfo, false)
	if err == nil {
		t.Cleanup(cleanup)
		t.Fatal("expected error for unwritable path, got nil")
	}
}
