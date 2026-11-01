package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// DeleteAdByIDFunc deletes a single ad. It fails with ErrorCodeNotFound when no ad matches the identifier or ErrForbidden if the user is not the owner of the ad.
	DeleteAdByIDFunc func(ctx context.Context, id domain.AdID, userID domain.UserID) error

	deleteAdByID struct {
		adDAO adsdao.DAO
	}
)

// NewDeleteAdByIDFunc returns a DeleteAdByIDFunc backed by the given DAO.
func NewDeleteAdByIDFunc(adDAO adsdao.DAO) DeleteAdByIDFunc {
	return deleteAdByID{adDAO: adDAO}.deleteAdByID
}

func (uc deleteAdByID) deleteAdByID(ctx context.Context, id domain.AdID, userID domain.UserID) error {
	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	ad, err := tx.GetAdByID(ctx, id)

	if err != nil {
		return fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	if userID != ad.UserID {
		return fmt.Errorf("unable to delete ad %s: user %s is not the owner: %w", id, userID, domain.ErrForbidden)
	}

	err = tx.DeleteAdByID(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	return nil
}
