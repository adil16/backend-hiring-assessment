package usecase

import (
	"context"
	"errors"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var ErrQuotaExceeded = errors.New("publication quota exceeded")

const maxDailyPublications = 5

type (
	// CreateAdFunc creates an ad owned by the calling user.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create.
	// OwnerID is the authenticated user.
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
	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	// Serialize publication attempts for this user.
	if err := tx.LockUserPublications(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to lock user publications: %w", err)
	}

	// Check the rolling 24-hour quota.
	count, err := tx.CountRecentPublications(ctx, req.OwnerID)
	if err != nil {
		return 0, fmt.Errorf("unable to count recent publications: %w", err)
	}

	if count >= maxDailyPublications {
		return 0, ErrQuotaExceeded
	}

	// Create the ad within the same transaction.
	adID, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
		Title:      req.Title,
		PriceCents: req.PriceCents,
		PhotoURL:   req.PhotoURL,
		OwnerID:    req.OwnerID,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	// Record the publication only for successful single-ad creation.
	if err := tx.RecordPublication(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to record publication: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("unable to commit transaction: %w", err)
	}

	return adID, nil
}
