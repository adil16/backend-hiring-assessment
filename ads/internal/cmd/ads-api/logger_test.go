package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureLogger(t *testing.T) {
	original := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	tests := []struct {
		name    string
		env     string
		want    slog.Level
		wantErr bool
	}{
		{name: "default level", env: "", want: slog.LevelInfo},
		{name: "debug level", env: "debug", want: slog.LevelDebug},
		{name: "info level", env: "info", want: slog.LevelInfo},
		{name: "warn level", env: "warn", want: slog.LevelWarn},
		{name: "error level", env: "error", want: slog.LevelError},
		{name: "case and whitespace", env: " DEBUG ", want: slog.LevelDebug},
		{name: "invalid level", env: "trace", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tt.env)

			err := configureLogger()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			for _, level := range []slog.Level{
				slog.LevelDebug,
				slog.LevelInfo,
				slog.LevelWarn,
				slog.LevelError,
			} {
				assert.Equal(
					t,
					level >= tt.want,
					slog.Default().Enabled(context.Background(), level),
					"unexpected filtering for level %s", level,
				)
			}
		})
	}
}
