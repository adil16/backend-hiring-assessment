package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(middleware.RequireValidUUIDUserID(handlerImportAds(fn))))
	return httptest.NewServer(mux)
}

func postImportAdsWithKey(
	t *testing.T,
	url string,
	body importAdsBody,
	userID string,
	key string,
) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewReader(payload),
	)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", userID)
	req.Header.Set("Idempotency-Key", key)

	resp, err := http.DefaultClient.Do(req)
	requireHTTPResponse(t, resp, err)

	return resp
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
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{Title: "bike", PriceCents: new(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: new(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
			{Title: "car", PriceCents: new(int64(200000)), PhotoURL: "https://example.com/car.jpg"},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "00000000-0000-0000-0000-000000000001")

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
		t.Cleanup(srv.Close)

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

		resp := postJSON(t, srv.URL+"/ads/import", body, "00000000-0000-0000-0000-000000000042")

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 1, calls)
		require.Len(t, received.Ads, 3)

		assert.Equal(t, int64(5000), received.Ads[0].PriceCents)
		assert.Equal(t, int64(8999), received.Ads[1].PriceCents)
		assert.Equal(t, int64(120050), received.Ads[2].PriceCents)

		for _, ad := range received.Ads {
			assert.Equal(t, domain.UserID("00000000-0000-0000-0000-000000000042"), ad.OwnerID)
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
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{Title: "bike", PriceCents: new(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: new(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA")

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 2)

		assert.Equal(t, domain.UserID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), received.OwnerID)
		assert.Equal(t, domain.UserID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), received.Ads[0].OwnerID)
		assert.Equal(t, domain.UserID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), received.Ads[1].OwnerID)
	})

	t.Run("returns 201 with empty IDs when body has no ads", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(
			t,
			srv.URL+"/ads/import",
			importAdsBody{Ads: []createAdBody{}},
			"00000000-0000-0000-0000-000000000001",
		)

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Zero(t, calls)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{}, got.IDs)
	})

	t.Run("trims title and photo URL for imported ads", func(t *testing.T) {
		var received usecase.ImportAdsRequest

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{1}, nil
		})
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "  mountain bike  ",
				PriceCents: new(int64(15000)),
				PhotoURL:   "  https://example.com/bike.jpg  ",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "00000000-0000-0000-0000-000000000001")

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 1)
		assert.Equal(t, "mountain bike", received.Ads[0].Title)
		assert.Equal(t, "https://example.com/bike.jpg", received.Ads[0].PhotoURL)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{}, "")

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 when Authorization header contains an invalid UUID", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{}, "nimportequoi")

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads/import", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "00000000-0000-0000-0000-000000000001")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 without calling usecase when one price is invalid", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

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
				Price:    ptrJSONNumber("12.345"),
				PhotoURL: "https://example.com/car.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "00000000-0000-0000-0000-000000000001")

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 when an ad has both price fields", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
			{
				Title:      "scooter",
				PriceCents: new(int64(8999)),
				Price:      ptrJSONNumber("89.99"),
				PhotoURL:   "https://example.com/scooter.jpg",
			},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "00000000-0000-0000-0000-000000000001")

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})

	t.Run("returns 400 when an ad has invalid fields", func(t *testing.T) {
		tests := []struct {
			name      string
			invalidAd createAdBody
		}{
			{
				name: "blank title",
				invalidAd: createAdBody{
					Title:      "   ",
					PriceCents: new(int64(8000)),
					PhotoURL:   "https://example.com/scooter.jpg",
				},
			},
			{
				name: "invalid photo URL",
				invalidAd: createAdBody{
					Title:      "scooter",
					PriceCents: new(int64(8000)),
					PhotoURL:   "not-a-url",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				calls := 0

				srv := newImportAdsServer(func(
					_ context.Context,
					_ usecase.ImportAdsRequest,
				) ([]domain.AdID, error) {
					calls++
					return nil, nil
				})
				t.Cleanup(srv.Close)

				body := importAdsBody{Ads: []createAdBody{
					{
						Title:      "bike",
						PriceCents: new(int64(5000)),
						PhotoURL:   "https://example.com/bike.jpg",
					},
					tt.invalidAd,
				}}

				resp := postJSON(
					t,
					srv.URL+"/ads/import",
					body,
					"00000000-0000-0000-0000-000000000001",
				)

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
				assert.Zero(t, calls)
			})
		}
	})

	t.Run("returns 500 when import usecase fails", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls++
			assert.Len(t, req.Ads, 3)
			return nil, errors.New("db down")
		})
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{Title: "bike", PriceCents: new(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: new(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
			{Title: "car", PriceCents: new(int64(200000)), PhotoURL: "https://example.com/car.jpg"},
		}}

		resp := postJSON(t, srv.URL+"/ads/import", body, "00000000-0000-0000-0000-000000000001")

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, 1, calls)
	})

	t.Run("passes idempotency key to usecase", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(
			_ context.Context,
			req usecase.ImportAdsRequest,
		) ([]domain.AdID, error) {
			calls++

			assert.Equal(t, "import-123", req.IdempotencyKey)
			assert.Equal(t, domain.UserID("00000000-0000-0000-0000-000000000042"), req.OwnerID)
			assert.Len(t, req.Ads, 1)

			if len(req.Ads) == 1 {
				assert.Equal(t, domain.UserID("00000000-0000-0000-0000-000000000042"), req.Ads[0].OwnerID)
			}

			return []domain.AdID{10}, nil
		})
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
		}}

		resp := postImportAdsWithKey(
			t,
			srv.URL+"/ads/import",
			body,
			"00000000-0000-0000-0000-000000000042",
			"import-123",
		)

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 1, calls)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{10}, got.IDs)
	})

	t.Run("returns 409 on idempotency conflict", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(
			_ context.Context,
			_ usecase.ImportAdsRequest,
		) ([]domain.AdID, error) {
			calls++
			return nil, usecase.ErrIdempotencyConflict
		})
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
		}}

		resp := postImportAdsWithKey(
			t,
			srv.URL+"/ads/import",
			body,
			"00000000-0000-0000-0000-000000000001",
			"import-123",
		)

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, 1, calls)
	})

	t.Run("records empty import when idempotency key is provided", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(
			_ context.Context,
			req usecase.ImportAdsRequest,
		) ([]domain.AdID, error) {
			calls++

			assert.Equal(t, "empty-import-123", req.IdempotencyKey)
			assert.Equal(t, domain.UserID("00000000-0000-0000-0000-000000000001"), req.OwnerID)
			assert.Empty(t, req.Ads)

			return []domain.AdID{}, nil
		})
		t.Cleanup(srv.Close)

		resp := postImportAdsWithKey(
			t,
			srv.URL+"/ads/import",
			importAdsBody{Ads: []createAdBody{}},
			"00000000-0000-0000-0000-000000000001",
			"empty-import-123",
		)

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 1, calls)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{}, got.IDs)
	})

	t.Run("returns 400 when idempotency key exceeds 255 characters", func(t *testing.T) {
		calls := 0

		srv := newImportAdsServer(func(
			_ context.Context,
			_ usecase.ImportAdsRequest,
		) ([]domain.AdID, error) {
			calls++
			return nil, nil
		})
		t.Cleanup(srv.Close)

		body := importAdsBody{Ads: []createAdBody{
			{
				Title:      "bike",
				PriceCents: new(int64(5000)),
				PhotoURL:   "https://example.com/bike.jpg",
			},
		}}

		resp := postImportAdsWithKey(
			t,
			srv.URL+"/ads/import",
			body,
			"00000000-0000-0000-0000-000000000001",
			strings.Repeat("a", 256),
		)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Zero(t, calls)
	})
}
