package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

type getAdResponse struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	PriceCents int64     `json:"price_cents"`
	PhotoURL   string    `json:"photo_url"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func adToGetAdResponse(a domain.Ad) getAdResponse {
	return getAdResponse{
		ID:         int64(a.ID),
		Title:      a.Title,
		PriceCents: a.Price,
		PhotoURL:   a.PhotoURL,
		UserID:     a.UserID.String(),
		CreatedAt:  a.CreatedAt.UTC(),
	}
}

func handlerGetAd(getAd usecase.GetAdByIDFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		ad, err := getAd(r.Context(), adID)
		if err != nil {
			if errors.Is(err, adsdao.ErrNotFound) {
				rw.WriteHeader(http.StatusNotFound)
			} else {
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(adToGetAdResponse(ad))
	}
}
