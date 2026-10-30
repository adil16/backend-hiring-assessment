package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func postJSON(t *testing.T, url string, body any, authHeader string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestHandlerCreateAd(t *testing.T) {
	validBody := createAdBody{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
	}

	t.Run("returns 201 with ID on success", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			return domain.AdID(7), nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

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
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-42")
		defer resp.Body.Close()

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
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 when body is invalid JSON", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads", bytes.NewBufferString("not json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, errors.New("db down")
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
