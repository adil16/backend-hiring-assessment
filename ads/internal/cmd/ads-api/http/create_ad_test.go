package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newCreateAdServer(fn usecase.CreateAdFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads", middleware.RequireAuth(handlerCreateAd(fn)))
	return httptest.NewServer(mux)
}

func TestHandlerCreateAd(t *testing.T) {
	validBody := createAdBody{
		Title:      "mountain bike",
		PriceCents: new(int64(15000)),
		PhotoURL:   "https://example.com/bike.jpg",
	}

	t.Run("returns 201 with ID on success", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			return domain.AdID(7), nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got createAdResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, int64(7), got.ID)
	})

	t.Run("passes authenticated user ID as owner to the usecase", func(t *testing.T) {
		var received usecase.CreateAdRequest
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			received = req
			return domain.AdID(1), nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-42")

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, domain.UserID("user-42"), received.OwnerID)
		assert.Equal(t, "mountain bike", received.Title)
		assert.Equal(t, int64(15000), received.PriceCents)
		assert.Equal(t, "https://example.com/bike.jpg", received.PhotoURL)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "")

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 when body is invalid JSON", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads", bytes.NewBufferString("not json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 429 when publication quota is exceeded", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, fmt.Errorf("unable to create ad: %w", usecase.ErrQuotaExceeded)
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, errors.New("db down")
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestHandlerCreateAd_Price(t *testing.T) {
	tests := []struct {
		name           string
		body           any
		wantStatus     int
		wantPriceCents int64
		wantCalled     bool
	}{
		{
			name: "accepts legacy price_cents",
			body: map[string]any{
				"title":       "bike",
				"price_cents": 1234,
			},
			wantStatus:     http.StatusCreated,
			wantPriceCents: 1234,
			wantCalled:     true,
		},
		{
			name: "accepts price in euros",
			body: map[string]any{
				"title": "bike",
				"price": json.Number("12.34"),
			},
			wantStatus:     http.StatusCreated,
			wantPriceCents: 1234,
			wantCalled:     true,
		},
		{
			name: "accepts zero euros",
			body: map[string]any{
				"title": "bike",
				"price": 0,
			},
			wantStatus:     http.StatusCreated,
			wantPriceCents: 0,
			wantCalled:     true,
		},
		{
			name: "rejects both price fields",
			body: map[string]any{
				"title":       "bike",
				"price":       12.34,
				"price_cents": 1234,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing price",
			body: map[string]any{
				"title": "bike",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects null price",
			body: map[string]any{
				"title": "bike",
				"price": nil,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects sub-cent precision",
			body: map[string]any{
				"title": "bike",
				"price": json.Number("12.345"),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects negative euros",
			body: map[string]any{
				"title": "bike",
				"price": -1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects price overflow",
			body: map[string]any{
				"title": "bike",
				"price": json.Number("92233720368547758.08"),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects non-numeric price",
			body: map[string]any{
				"title": "bike",
				"price": "not-a-number",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false

			srv := newCreateAdServer(func(
				_ context.Context,
				req usecase.CreateAdRequest,
			) (domain.AdID, error) {
				called = true
				assert.Equal(t, tt.wantPriceCents, req.PriceCents)
				return domain.AdID(7), nil
			})
			t.Cleanup(srv.Close)

			resp := postJSON(t, srv.URL+"/ads", tt.body, "user-1")

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, tt.wantCalled, called)
		})
	}
}

func TestCreateAdBody_PriceInCents(t *testing.T) {
	tests := []struct {
		name      string
		body      createAdBody
		wantCents int64
		wantErr   bool
	}{
		{
			name: "legacy price in cents",
			body: createAdBody{
				PriceCents: ptrInt64(1234),
			},
			wantCents: 1234,
		},
		{
			name: "legacy zero cents",
			body: createAdBody{
				PriceCents: ptrInt64(0),
			},
			wantCents: 0,
		},
		{
			name: "euros with two decimals",
			body: createAdBody{
				Price: ptrJSONNumber("12.34"),
			},
			wantCents: 1234,
		},
		{
			name: "euros with one decimal",
			body: createAdBody{
				Price: ptrJSONNumber("12.3"),
			},
			wantCents: 1230,
		},
		{
			name: "euros with trailing zeros",
			body: createAdBody{
				Price: ptrJSONNumber("12.3400"),
			},
			wantCents: 1234,
		},
		{
			name: "one cent",
			body: createAdBody{
				Price: ptrJSONNumber("0.01"),
			},
			wantCents: 1,
		},
		{
			name: "zero euros",
			body: createAdBody{
				Price: ptrJSONNumber("0"),
			},
			wantCents: 0,
		},
		{
			name: "maximum representable cents",
			body: createAdBody{
				Price: ptrJSONNumber("92233720368547758.07"),
			},
			wantCents: int64(9223372036854775807),
		},
		{
			name: "both price fields provided",
			body: createAdBody{
				Price:      ptrJSONNumber("12.34"),
				PriceCents: ptrInt64(1234),
			},
			wantErr: true,
		},
		{
			name:    "missing price",
			body:    createAdBody{},
			wantErr: true,
		},
		{
			name: "fraction of a cent",
			body: createAdBody{
				Price: ptrJSONNumber("12.345"),
			},
			wantErr: true,
		},
		{
			name: "negative euros",
			body: createAdBody{
				Price: ptrJSONNumber("-1"),
			},
			wantErr: true,
		},
		{
			name: "overflow",
			body: createAdBody{
				Price: ptrJSONNumber("92233720368547758.08"),
			},
			wantErr: true,
		},
		{
			name: "invalid decimal",
			body: createAdBody{
				Price: ptrJSONNumber("not-a-number"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.body.priceInCents()

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantCents, got)
		})
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

func ptrJSONNumber(v string) *json.Number {
	n := json.Number(v)
	return &n
}
