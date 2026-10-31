package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestDeleteAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("DeleteAdByID", ctx, domain.AdID(42)).Return(nil)

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 42)

		require.NoError(t, err)
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("DeleteAdByID", ctx, mock.Anything).Return(adsdao.ErrNotFound)

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")
		m.On("DeleteAdByID", ctx, mock.Anything).Return(dbErr)

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})
}
