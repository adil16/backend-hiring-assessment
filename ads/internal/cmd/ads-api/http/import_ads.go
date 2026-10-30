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

func handlerImportAds(createAd usecase.CreateAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body importAdsBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		ownerID := domain.UserID(middleware.UserIDFromRequest(r))
		ids := make([]int64, 0, len(body.Ads))

		for _, item := range body.Ads {
			adID, err := createAd(r.Context(), usecase.CreateAdRequest{
				Title:      item.Title,
				PriceCents: item.PriceCents,
				PhotoURL:   item.PhotoURL,
				OwnerID:    ownerID,
			})
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			ids = append(ids, int64(adID))
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(importAdsResponse{IDs: ids})
	}
}
