package http

import (
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

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(fn)))
	return httptest.NewServer(mux)
}

func TestHandlerImportAds(t *testing.T) {
	t.Run("returns 201 with all generated IDs on success", func(t *testing.T) {
		ids := []domain.AdID{10, 11, 12}
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Len(t, req.Ads, 3)
			return ids, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{Title: "bike", PriceCents: new(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: new(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
			{Title: "car", PriceCents: new(int64(200000)), PhotoURL: "https://example.com/car.jpg"},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 1, calls)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{10, 11, 12}, got.IDs)
	})

	t.Run("accepts both legacy and new price formats", func(t *testing.T) {
		var received usecase.ImportAdsRequest
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			received = req
			return []domain.AdID{1, 2, 3}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
			{
				Title:    "scooter",
				Price:    ptrJSONNumber("89.99"),
				PhotoURL: "https://example.com/scooter.jpg",
			},
			{
				Title:    "car",
				Price:    ptrJSONNumber("1200.50"),
				PhotoURL: "https://example.com/car.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-42")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 1, calls)
		require.Len(t, received.Ads, 3)

		assert.Equal(t, int64(5000), received.Ads[0].PriceCents)
		assert.Equal(t, int64(8999), received.Ads[1].PriceCents)
		assert.Equal(t, int64(120050), received.Ads[2].PriceCents)

		for _, ad := range received.Ads {
			assert.Equal(t, domain.UserID("user-42"), ad.OwnerID)
		}

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{1, 2, 3}, got.IDs)
	})

	t.Run("passes authenticated user ID as owner for every ad", func(t *testing.T) {
		var received usecase.ImportAdsRequest

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{1, 2}, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{Title: "bike", PriceCents: new(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: new(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-42")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 2)

		assert.Equal(t, domain.UserID("user-42"), received.Ads[0].OwnerID)
		assert.Equal(t, domain.UserID("user-42"), received.Ads[1].OwnerID)
	})

	t.Run("returns 201 with empty IDs when body has no ads", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Empty(t, req.Ads)
			return []domain.AdID{}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{Ads: []createAdBody{}}, "user-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Zero(t, calls)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Empty(t, got.IDs)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{}, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		defer srv.Close()

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads/import", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 without calling usecase when one price is invalid", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
			},
			{
				Title: "scooter",
				Price: ptrJSONNumber("89.99"),
			},
			{
				Title: "car",
				Price: ptrJSONNumber("12.345"),
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 when an ad has both price fields", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
			},
			{
				Title:      "scooter",
				PriceCents: new(int64(8999)),
				Price:      ptrJSONNumber("89.99"),
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 500 when import usecase fails", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Len(t, req.Ads, 3)
			return nil, errors.New("db down")
		})
		defer srv.Close()

		body := importAdsBody{Ads: []createAdBody{
			{Title: "bike", PriceCents: new(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: new(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
			{Title: "car", PriceCents: new(int64(200000)), PhotoURL: "https://example.com/car.jpg"},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, 1, calls)
	})
}
