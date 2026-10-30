package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func newGetAdServer(fn usecase.GetAdByIDFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /ads/{adID}", handlerGetAd(fn))

	return httptest.NewServer(mux)
}

func TestHandlerGetAd(t *testing.T) {
	createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("returns 200 with ad JSON on success", func(t *testing.T) {
		ad := domain.Ad{
			ID:        42,
			Title:     "mountain bike",
			Price:     15000,
			PhotoURL:  "https://example.com/bike.jpg",
			UserID:    "user-1",
			CreatedAt: createdAt,
		}
		srv := newGetAdServer(func(_ context.Context, id domain.AdID) (domain.Ad, error) {
			return ad, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/42")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got getAdResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, int64(42), got.ID)
		assert.Equal(t, "mountain bike", got.Title)
		assert.Equal(t, int64(15000), got.PriceCents)
		assert.Equal(t, "https://example.com/bike.jpg", got.PhotoURL)
		assert.Equal(t, "user-1", got.UserID)
		assert.Equal(t, createdAt, got.CreatedAt)
	})

	t.Run("returns 400 when adID is not a number", func(t *testing.T) {
		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.Ad, error) {
			t.Fatal("usecase should not be called")
			return domain.Ad{}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/not-a-number")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 404 when ad does not exist", func(t *testing.T) {
		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.Ad, error) {
			return domain.Ad{}, adsdao.ErrNotFound
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/99")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns 500 on unexpected usecase error", func(t *testing.T) {
		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.Ad, error) {
			return domain.Ad{}, errors.New("db down")
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/1")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("passes the correct ad ID to the usecase", func(t *testing.T) {
		var receivedID domain.AdID
		srv := newGetAdServer(func(_ context.Context, id domain.AdID) (domain.Ad, error) {
			receivedID = id
			return domain.Ad{ID: id}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/7")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, domain.AdID(7), receivedID)
	})
}
