package http

import (
	"encoding/json"
	"net/http"

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

		if len(body.Ads) == 0 {
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
			priceCents, err := item.priceInCents()
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			ads = append(ads, usecase.CreateAdRequest{
				Title:      item.Title,
				PriceCents: priceCents,
				PhotoURL:   item.PhotoURL,
				OwnerID:    ownerID,
			})
		}

		adIDs, err := importAds(r.Context(), usecase.ImportAdsRequest{
			Ads: ads,
		})
		if err != nil {
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
