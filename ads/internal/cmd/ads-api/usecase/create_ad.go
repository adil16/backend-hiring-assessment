package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// CreateAdFunc creates an ad owned by the calling user.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create. OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	createAd struct {
		adDAO adsdao.DAO
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given DAO.
func NewCreateAdFunc(adDAO adsdao.DAO) CreateAdFunc {
	return createAd{adDAO: adDAO}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	adID, err := uc.adDAO.CreateAd(ctx, adsdao.CreateAdRequest{
		Title:      req.Title,
		PriceCents: req.PriceCents,
		PhotoURL:   req.PhotoURL,
		OwnerID:    req.OwnerID,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return adID, nil
}
