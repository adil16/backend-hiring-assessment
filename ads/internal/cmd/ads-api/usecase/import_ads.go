package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// ImportAdsFunc creates multiple ads atomically.
	// Either all ads are created or none are.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// ImportAdsRequest describes the ads to import.
	ImportAdsRequest struct {
		Ads []CreateAdRequest
	}

	importAds struct {
		adDAO adsdao.DAO
	}
)

// NewImportAdsFunc returns an ImportAdsFunc backed by the given DAO.
func NewImportAdsFunc(adDAO adsdao.DAO) ImportAdsFunc {
	return importAds{adDAO: adDAO}.importAds
}

func (uc importAds) importAds(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error) {
	if len(req.Ads) == 0 {
		return []domain.AdID{}, nil
	}

	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	ids := make([]domain.AdID, 0, len(req.Ads))

	for i, ad := range req.Ads {
		id, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
			Title:      ad.Title,
			PriceCents: ad.PriceCents,
			PhotoURL:   ad.PhotoURL,
			OwnerID:    ad.OwnerID,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to import ad at index %d: %w", i, err)
		}

		ids = append(ids, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("unable to commit transaction: %w", err)
	}

	return ids, nil
}
