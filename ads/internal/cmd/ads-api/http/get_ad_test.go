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
		ad := domain.AdWithUsername{
			Ad: domain.Ad{
				ID:        42,
				Title:     "mountain bike",
				Price:     15000,
				PhotoURL:  "https://example.com/bike.jpg",
				UserID:    "user-1",
				CreatedAt: createdAt,
			},
			Username: "alice",
		}
		srv := newGetAdServer(func(_ context.Context, id domain.AdID) (domain.AdWithUsername, error) {
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
		assert.Equal(t, "150.00", got.Price.String())
		assert.Equal(t, "https://example.com/bike.jpg", got.PhotoURL)
		assert.Equal(t, "user-1", got.UserID)
		assert.Equal(t, "alice", got.Username)
		assert.Equal(t, createdAt, got.CreatedAt)
	})

	t.Run("returns 400 when adID is not a number", func(t *testing.T) {
		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.AdWithUsername, error) {
			t.Fatal("usecase should not be called")
			return domain.AdWithUsername{}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/not-a-number")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 404 when ad does not exist", func(t *testing.T) {
		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.AdWithUsername, error) {
			return domain.AdWithUsername{}, adsdao.ErrNotFound
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/99")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns 200 without username when user does not exist", func(t *testing.T) {
		ad := domain.Ad{
			ID:     99,
			Title:  "bike",
			Price:  5000,
			UserID: "user-1",
		}

		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.AdWithUsername, error) {
			return domain.AdWithUsername{Ad: ad}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/99")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

		assert.Equal(t, float64(99), body["id"])
		assert.Equal(t, "bike", body["title"])
		assert.Equal(t, "user-1", body["user_id"])
		assert.Equal(t, float64(50), body["price"])
		assert.Equal(t, float64(5000), body["price_cents"])
		assert.NotContains(t, body, "username")
	})

	t.Run("returns price as a JSON number alongside price_cents", func(t *testing.T) {
		ad := domain.AdWithUsername{
			Ad: domain.Ad{
				ID:    42,
				Title: "bike",
				Price: 1234,
			},
		}

		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.AdWithUsername, error) {
			return ad, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/42")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]json.RawMessage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

		assert.JSONEq(t, `12.34`, string(body["price"]))
		assert.JSONEq(t, `1234`, string(body["price_cents"]))
	})

	t.Run("returns 500 on unexpected usecase error", func(t *testing.T) {
		srv := newGetAdServer(func(_ context.Context, _ domain.AdID) (domain.AdWithUsername, error) {
			return domain.AdWithUsername{}, errors.New("db down")
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/1")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("passes the correct ad ID to the usecase", func(t *testing.T) {
		var receivedID domain.AdID
		srv := newGetAdServer(func(_ context.Context, id domain.AdID) (domain.AdWithUsername, error) {
			receivedID = id
			return domain.AdWithUsername{Ad: domain.Ad{ID: id}}, nil
		})
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/ads/7")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, domain.AdID(7), receivedID)
	})
}
