package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type importAdsBody struct {
	Ads []createAdBody `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body importAdsBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		idempotencyKey := r.Header.Get("Idempotency-Key")
		if len(idempotencyKey) > 255 {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		// An empty import without an idempotency key
		// does not need to reach the use case.
		if len(body.Ads) == 0 && idempotencyKey == "" {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(rw).Encode(importAdsResponse{
				IDs: []int64{},
			})
			return
		}

		ownerID := domain.UserID(middleware.UserIDFromRequest(r))
		ads := make([]usecase.CreateAdRequest, 0, len(body.Ads))

		for _, item := range body.Ads {
			if err := item.validateFields(); err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			priceCents, err := item.priceInCents()
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			ads = append(ads, usecase.CreateAdRequest{
				Title:      strings.TrimSpace(item.Title),
				PriceCents: priceCents,
				PhotoURL:   strings.TrimSpace(item.PhotoURL),
				OwnerID:    ownerID,
			})
		}

		adIDs, err := importAds(r.Context(), usecase.ImportAdsRequest{
			Ads:            ads,
			OwnerID:        ownerID,
			IdempotencyKey: idempotencyKey,
		})
		if err != nil {
			if errors.Is(err, usecase.ErrIdempotencyConflict) {
				rw.WriteHeader(http.StatusConflict)
				return
			}

			slog.ErrorContext(r.Context(),
				"failed to import ads",
				"operation", "import_ads",
				"error", err,
			)

			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		ids := make([]int64, 0, len(adIDs))
		for _, id := range adIDs {
			ids = append(ids, int64(id))
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(importAdsResponse{IDs: ids})
	}
}
