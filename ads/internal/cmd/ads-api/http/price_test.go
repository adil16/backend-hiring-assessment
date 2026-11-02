package http

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCentsToPrice(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  string
	}{
		{
			name:  "zero",
			cents: 0,
			want:  "0.00",
		},
		{
			name:  "one cent",
			cents: 1,
			want:  "0.01",
		},
		{
			name:  "less than one euro",
			cents: 99,
			want:  "0.99",
		},
		{
			name:  "one euro",
			cents: 100,
			want:  "1.00",
		},
		{
			name:  "euros and cents",
			cents: 1234,
			want:  "12.34",
		},
		{
			name:  "whole euros",
			cents: 15000,
			want:  "150.00",
		},
		{
			name:  "negative cents",
			cents: -50,
			want:  "-0.50",
		},
		{
			name:  "negative euros",
			cents: -1234,
			want:  "-12.34",
		},
		{
			name:  "maximum int64",
			cents: math.MaxInt64,
			want:  "92233720368547758.07",
		},
		{
			name:  "minimum int64",
			cents: math.MinInt64,
			want:  "-92233720368547758.08",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := centsToPrice(tt.cents)

			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestCentsToPrice_JSON(t *testing.T) {
	got, err := json.Marshal(struct {
		Price json.Number `json:"price"`
	}{
		Price: centsToPrice(1234),
	})

	require.NoError(t, err)
	assert.JSONEq(t, `{"price":12.34}`, string(got))
	assert.Contains(t, string(got), `"price":12.34`)
	assert.NotContains(t, string(got), `"price":"12.34"`)
}
