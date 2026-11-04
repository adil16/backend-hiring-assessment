package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func configureLogger() error {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))

	level := slog.LevelInfo

	switch raw {
	case "", "info":
		// Default level.
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return fmt.Errorf("invalid LOG_LEVEL %q", raw)
	}

	logger := slog.New(slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: level,
		},
	))

	slog.SetDefault(logger)
	return nil
}
