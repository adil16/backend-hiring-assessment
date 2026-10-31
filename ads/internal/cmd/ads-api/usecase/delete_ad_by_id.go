package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// DeleteAdByIDFunc deletes a single ad. It fails with ErrorCodeNotFound when no ad matches the identifier.
	DeleteAdByIDFunc func(ctx context.Context, id domain.AdID) error

	deleteAdByID struct {
		adDAO adsdao.DAO
	}
)

// NewDeleteAdByIDFunc returns a DeleteAdByIDFunc backed by the given DAO.
func NewDeleteAdByIDFunc(adDAO adsdao.DAO) DeleteAdByIDFunc {
	return deleteAdByID{adDAO: adDAO}.deleteAdByID
}

func (uc deleteAdByID) deleteAdByID(ctx context.Context, id domain.AdID) error {
	err := uc.adDAO.DeleteAdByID(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", id, err)
	}

	return nil
}
