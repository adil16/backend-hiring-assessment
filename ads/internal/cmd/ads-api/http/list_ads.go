package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

type listAdsItem struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	PriceCents int64     `json:"price_cents"`
	PhotoURL   string    `json:"photo_url"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type listAdsResponse struct {
	Ads []listAdsItem `json:"ads"`
}

func adToListAdsItem(a domain.Ad) listAdsItem {
	return listAdsItem{
		ID:         int64(a.ID),
		Title:      a.Title,
		PriceCents: a.Price,
		PhotoURL:   a.PhotoURL,
		UserID:     a.UserID.String(),
		CreatedAt:  a.CreatedAt.UTC(),
	}
}

func handlerListAds(listAds usecase.ListAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		req := usecase.ListAdsRequest{
			Title: q.Get("title"),
		}

		if raw := q.Get("user_id"); raw != "" {
			ownerID := domain.UserID(raw)
			req.OwnerID = ownerID
		}

		if raw := q.Get("min_price_cents"); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			req.MinPriceCents = v
		}

		if raw := q.Get("max_price_cents"); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			req.MaxPriceCents = v
		}

		ads, err := listAds(r.Context(), req)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		items := make([]listAdsItem, 0, len(ads))
		for _, a := range ads {
			items = append(items, adToListAdsItem(a))
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(listAdsResponse{Ads: items})
	}
}
