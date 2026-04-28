package logger

import (
	"io"
	"log/slog"
	"os"
)

func New(logPath string, level slog.Level, dev bool) (*slog.Logger, func(), error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	var w io.Writer = logFile
	if dev {
		w = io.MultiWriter(os.Stdout, logFile)
	}

	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}))

	cleanup := func() { logFile.Close() }
	return logger, cleanup, nil
}
