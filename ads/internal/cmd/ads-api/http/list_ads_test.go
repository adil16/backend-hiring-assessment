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
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func newListAdsServer(fn usecase.ListAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /ads", handlerListAds(fn))
	return httptest.NewServer(mux)
}

func TestHandlerListAds(t *testing.T) {
	createdAt := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

	t.Run("returns 200 with ads JSON on success", func(t *testing.T) {
		ads := []domain.Ad{
			{ID: 1, Title: "bike", Price: 5000, PhotoURL: "https://example.com/bike.jpg", UserID: "00000000-0000-0000-0000-000000000001", CreatedAt: createdAt},
			{ID: 2, Title: "scooter", Price: 12000, PhotoURL: "https://example.com/scooter.jpg", UserID: "00000000-0000-0000-0000-000000000002", CreatedAt: createdAt},
		}
		srv := newListAdsServer(func(_ context.Context, _ usecase.ListAdsRequest) ([]domain.Ad, error) {
			return ads, nil
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads")
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got listAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		require.Len(t, got.Ads, 2)

		// First ad
		assert.Equal(t, int64(1), got.Ads[0].ID)
		assert.Equal(t, "bike", got.Ads[0].Title)
		assert.Equal(t, int64(5000), got.Ads[0].PriceCents)
		assert.Equal(t, "50.00", got.Ads[0].Price.String())
		assert.Equal(t, "https://example.com/bike.jpg", got.Ads[0].PhotoURL)
		assert.Equal(t, "00000000-0000-0000-0000-000000000001", got.Ads[0].UserID)
		assert.Equal(t, createdAt, got.Ads[0].CreatedAt)

		// Second ad
		assert.Equal(t, int64(2), got.Ads[1].ID)
		assert.Equal(t, "scooter", got.Ads[1].Title)
		assert.Equal(t, int64(12000), got.Ads[1].PriceCents)
		assert.Equal(t, "120.00", got.Ads[1].Price.String())
		assert.Equal(t, "https://example.com/scooter.jpg", got.Ads[1].PhotoURL)
		assert.Equal(t, "00000000-0000-0000-0000-000000000002", got.Ads[1].UserID)
		assert.Equal(t, createdAt, got.Ads[1].CreatedAt)
	})

	t.Run("returns 200 with empty ads array when no results", func(t *testing.T) {
		srv := newListAdsServer(func(_ context.Context, _ usecase.ListAdsRequest) ([]domain.Ad, error) {
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads")
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var got listAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Empty(t, got.Ads)
	})

	t.Run("forwards query params to the usecase", func(t *testing.T) {
		var received usecase.ListAdsRequest
		srv := newListAdsServer(func(_ context.Context, req usecase.ListAdsRequest) ([]domain.Ad, error) {
			received = req
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads?title=bike&user_id=AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA&min_price_cents=1000&max_price_cents=50000")
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "bike", received.Title)
		assert.Equal(t, domain.UserID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), received.OwnerID)
		assert.Equal(t, int64(1000), received.MinPriceCents)
		assert.Equal(t, int64(50000), received.MaxPriceCents)
	})

	t.Run("returns price as a JSON number alongside price_cents", func(t *testing.T) {
		ads := []domain.Ad{
			{ID: 1, Title: "bike", Price: 1234},
		}

		srv := newListAdsServer(func(_ context.Context, _ usecase.ListAdsRequest) ([]domain.Ad, error) {
			return ads, nil
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads")
		requireHTTPResponse(t, resp, err)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Ads []struct {
				Price      json.RawMessage `json:"price"`
				PriceCents int64           `json:"price_cents"`
			} `json:"ads"`
		}

		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Len(t, body.Ads, 1)

		assert.Equal(t, `12.34`, string(body.Ads[0].Price))
		assert.Equal(t, int64(1234), body.Ads[0].PriceCents)
	})

	t.Run("returns 400 when min_price_cents is not a number", func(t *testing.T) {
		srv := newListAdsServer(func(_ context.Context, _ usecase.ListAdsRequest) ([]domain.Ad, error) {
			t.Fatal("usecase should not be called")
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads?min_price_cents=not-a-number")
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when max_price_cents is not a number", func(t *testing.T) {
		srv := newListAdsServer(func(_ context.Context, _ usecase.ListAdsRequest) ([]domain.Ad, error) {
			t.Fatal("usecase should not be called")
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads?max_price_cents=abc")
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 for invalid query filters", func(t *testing.T) {
		tests := []struct {
			name  string
			query string
		}{
			{
				name:  "invalid user ID",
				query: "?user_id=nimportequoi",
			},
			{
				name:  "negative minimum price",
				query: "?min_price_cents=-1",
			},
			{
				name:  "negative maximum price",
				query: "?max_price_cents=-100",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				calls := 0

				srv := newListAdsServer(func(
					_ context.Context,
					_ usecase.ListAdsRequest,
				) ([]domain.Ad, error) {
					calls++
					return nil, nil
				})
				t.Cleanup(srv.Close)

				resp, err := http.Get(srv.URL + "/ads" + tt.query)
				requireHTTPResponse(t, resp, err)

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
				assert.Zero(t, calls)
			})
		}
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newListAdsServer(func(_ context.Context, _ usecase.ListAdsRequest) ([]domain.Ad, error) {
			return nil, errors.New("db down")
		})
		t.Cleanup(srv.Close)

		resp, err := http.Get(srv.URL + "/ads")
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
