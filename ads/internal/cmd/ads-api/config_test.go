package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestText(t *testing.T) {
	const name = "ADS_TEST_TEXT"

	t.Run("returns environment value", func(t *testing.T) {
		t.Setenv(name, "custom")

		assert.Equal(t, "custom", text(name, "default"))
	})

	t.Run("returns fallback when environment value is empty", func(t *testing.T) {
		t.Setenv(name, "")

		assert.Equal(t, "default", text(name, "default"))
	})
}

func TestDuration(t *testing.T) {
	const name = "ADS_TEST_DURATION"
	const fallback = 2 * time.Second

	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{
			name: "valid duration",
			env:  "250ms",
			want: 250 * time.Millisecond,
		},
		{
			name: "invalid duration",
			env:  "invalid",
			want: fallback,
		},
		{
			name: "empty duration",
			env:  "",
			want: fallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(name, tt.env)

			assert.Equal(t, tt.want, duration(name, fallback))
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("returns default configuration", func(t *testing.T) {
		t.Setenv("HTTP_ADDR", "")
		t.Setenv("DATABASE_URL", "")
		t.Setenv("IDENTITY_URL", "")
		t.Setenv("IDENTITY_TIMEOUT", "")

		want := Config{
			HTTPAddr:        ":8080",
			DatabaseURL:     "postgres://ads:ads@localhost:5432/ads?sslmode=disable",
			IdentityURL:     "http://localhost:8081",
			IdentityTimeout: 2 * time.Second,
		}

		assert.Equal(t, want, Load())
	})

	t.Run("returns configured values", func(t *testing.T) {
		t.Setenv("HTTP_ADDR", ":9090")
		t.Setenv("DATABASE_URL", "postgres://localhost:5432/test")
		t.Setenv("IDENTITY_URL", "http://users:8081")
		t.Setenv("IDENTITY_TIMEOUT", "500ms")

		want := Config{
			HTTPAddr:        ":9090",
			DatabaseURL:     "postgres://localhost:5432/test",
			IdentityURL:     "http://users:8081",
			IdentityTimeout: 500 * time.Millisecond,
		}

		assert.Equal(t, want, Load())
	})
}
