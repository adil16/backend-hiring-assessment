package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

const (
	maxTitleLength    = 200
	maxPhotoURLLength = 2048
)

type createAdBody struct {
	Title      string       `json:"title"`
	PriceCents *int64       `json:"price_cents"`
	Price      *json.Number `json:"price"`
	PhotoURL   string       `json:"photo_url"`
}

type createAdResponse struct {
	ID int64 `json:"id"`
}

// handlerCreateAd serves POST /ads.
func handlerCreateAd(createAd usecase.CreateAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body createAdBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := body.validateFields(); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		priceCents, err := body.priceInCents()
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		adID, err := createAd(r.Context(), usecase.CreateAdRequest{
			Title:      strings.TrimSpace(body.Title),
			PriceCents: priceCents,
			PhotoURL:   strings.TrimSpace(body.PhotoURL),
			OwnerID:    domain.UserID(middleware.UserIDFromRequest(r)),
		})
		if err != nil {
			if errors.Is(err, usecase.ErrQuotaExceeded) {
				http.Error(rw, "publication quota exceeded", http.StatusTooManyRequests)
				return
			}

			slog.ErrorContext(r.Context(),
				"failed to create ad",
				"operation", "create_ad",
				"error", err,
			)

			http.Error(rw, "internal server error", http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(createAdResponse{ID: int64(adID)})
	}
}

func (b createAdBody) priceInCents() (int64, error) {
	if b.PriceCents != nil && b.Price != nil {
		return 0, errors.New("price and price_cents cannot both be provided")
	}

	if b.PriceCents != nil {
		if *b.PriceCents < 0 {
			return 0, errors.New("price cannot be negative")
		}
		return *b.PriceCents, nil
	}

	if b.Price == nil {
		return 0, errors.New("missing price")
	}

	price, ok := new(big.Rat).SetString(b.Price.String())
	if !ok {
		return 0, errors.New("invalid price")
	}

	if price.Sign() < 0 {
		return 0, errors.New("price cannot be negative")
	}

	cents := new(big.Rat).Mul(price, big.NewRat(100, 1))

	if !cents.IsInt() || !cents.Num().IsInt64() {
		return 0, errors.New("price must be representable in cents")
	}

	return cents.Num().Int64(), nil
}

func (b createAdBody) validateFields() error {
	title := strings.TrimSpace(b.Title)
	photoURL := strings.TrimSpace(b.PhotoURL)

	if title == "" {
		return errors.New("title cannot be empty")
	}

	if utf8.RuneCountInString(title) > maxTitleLength {
		return errors.New("title is too long")
	}

	if photoURL == "" {
		return errors.New("photo URL is required")
	}

	if len(photoURL) > maxPhotoURLLength {
		return errors.New("photo URL too long")
	}

	u, err := url.Parse(photoURL)
	if err != nil || !u.IsAbs() {
		return errors.New("invalid photo URL")
	}

	if (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() == "" {
		return errors.New("invalid photo URL")
	}

	return nil
}
