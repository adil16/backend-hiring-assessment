package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newCreateAdServer(fn usecase.CreateAdFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads", middleware.RequireAuth(middleware.RequireValidUUIDUserID(handlerCreateAd(fn))))
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

		resp := postJSON(t, srv.URL+"/ads", validBody, "00000000-0000-0000-0000-000000000001")

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

		resp := postJSON(t, srv.URL+"/ads", validBody, "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA")

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, domain.UserID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), received.OwnerID)
		assert.Equal(t, "mountain bike", received.Title)
		assert.Equal(t, int64(15000), received.PriceCents)
		assert.Equal(t, "https://example.com/bike.jpg", received.PhotoURL)
	})

	t.Run("trims title and photo URL before calling usecase", func(t *testing.T) {
		var received usecase.CreateAdRequest

		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			received = req
			return domain.AdID(1), nil
		})
		t.Cleanup(srv.Close)

		body := createAdBody{
			Title:      "  mountain bike  ",
			PriceCents: new(int64(15000)),
			PhotoURL:   "  https://example.com/bike.jpg  ",
		}

		resp := postJSON(t, srv.URL+"/ads", body, "00000000-0000-0000-0000-000000000001")

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "mountain bike", received.Title)
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

	t.Run("returns 400 when Authorization header contains an invalid UUID", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Error("usecase should not be called")
			return 0, nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "invaliduser")

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
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
		req.Header.Set("Authorization", "00000000-0000-0000-0000-000000000001")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 429 when publication quota is exceeded", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, fmt.Errorf("unable to create ad: %w", usecase.ErrQuotaExceeded)
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "00000000-0000-0000-0000-000000000001")

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, errors.New("db down")
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads", validBody, "00000000-0000-0000-0000-000000000001")

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestHandlerCreateAd_Price(t *testing.T) {
	const photoURL = "https://example.com/bike.jpg"

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
				"photo_url":   photoURL,
				"price_cents": 1234,
			},
			wantStatus:     http.StatusCreated,
			wantPriceCents: 1234,
			wantCalled:     true,
		},
		{
			name: "accepts price in euros",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     json.Number("12.34"),
			},
			wantStatus:     http.StatusCreated,
			wantPriceCents: 1234,
			wantCalled:     true,
		},
		{
			name: "accepts zero euros",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     0,
			},
			wantStatus:     http.StatusCreated,
			wantPriceCents: 0,
			wantCalled:     true,
		},
		{
			name: "rejects both price fields",
			body: map[string]any{
				"title":       "bike",
				"photo_url":   photoURL,
				"price":       12.34,
				"price_cents": 1234,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects missing price",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects null price",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     nil,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects sub-cent precision",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     json.Number("12.345"),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects negative euros",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     -1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects negative legacy price_cents",
			body: map[string]any{
				"title":       "bike",
				"photo_url":   photoURL,
				"price_cents": -100,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects price overflow",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     json.Number("92233720368547758.08"),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rejects non-numeric price",
			body: map[string]any{
				"title":     "bike",
				"photo_url": photoURL,
				"price":     "not-a-number",
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

			resp := postJSON(t, srv.URL+"/ads", tt.body, "00000000-0000-0000-0000-000000000001")

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
			name: "negative legacy price in cents",
			body: createAdBody{
				PriceCents: ptrInt64(-100),
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

func TestCreateAdBody_ValidateFields(t *testing.T) {
	const validURL = "https://example.com/bike.jpg"
	const urlPrefix = "https://example.com/"

	tests := []struct {
		name     string
		title    string
		photoURL string
		wantErr  bool
	}{
		{"valid fields", "bike", validURL, false},
		{"title at maximum length", strings.Repeat("é", 200), validURL, false},
		{"empty title", "", validURL, true},
		{"blank title", " \t ", validURL, true},
		{"title too long", strings.Repeat("é", 201), validURL, true},
		{"empty photo URL", "bike", "", true},
		{"relative photo URL", "bike", "/photos/bike.jpg", true},
		{"S3 URI", "bike", "s3://bucket/bike.jpg", false},
		{"FTP URI", "bike", "ftp://example.com/bike.jpg", false},
		{"URL without hostname", "bike", "https:///bike.jpg", true},
		{
			"photo URL at maximum length",
			"bike",
			urlPrefix + strings.Repeat("a", 2048-len(urlPrefix)),
			false,
		},
		{
			"photo URL too long",
			"bike",
			urlPrefix + strings.Repeat("a", 2049-len(urlPrefix)),
			true,
		},
		{
			"title at maximum length with spaces",
			"  " + strings.Repeat("é", 200) + "  ",
			validURL,
			false,
		},
		{
			"photo URL at maximum length with spaces",
			"bike",
			"  " + urlPrefix + strings.Repeat("a", 2048-len(urlPrefix)) + "  ",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := createAdBody{
				Title:    tt.title,
				PhotoURL: tt.photoURL,
			}

			err := body.validateFields()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestHandlerCreateAd_InvalidFields(t *testing.T) {
	const validURL = "https://example.com/bike.jpg"

	tests := []struct {
		name     string
		title    string
		photoURL string
	}{
		{"empty title", "", validURL},
		{"blank title", "   ", validURL},
		{"title too long", strings.Repeat("a", 201), validURL},
		{"invalid photo URL", "bike", "not-a-url"},
		{"photo URL too long", "bike", "https://example.com/" + strings.Repeat("a", 2048)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false

			srv := newCreateAdServer(func(
				_ context.Context,
				_ usecase.CreateAdRequest,
			) (domain.AdID, error) {
				called = true
				return 7, nil
			})
			t.Cleanup(srv.Close)

			body := map[string]any{
				"title":     tt.title,
				"price":     12.34,
				"photo_url": tt.photoURL,
			}

			resp := postJSON(
				t,
				srv.URL+"/ads",
				body,
				"00000000-0000-0000-0000-000000000001",
			)

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.False(t, called, "use case must not be called")
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
