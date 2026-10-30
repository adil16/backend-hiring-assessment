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

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)
		want := domain.Ad{ID: 42, Title: "bike", Price: 5000, UserID: "user-1"}
		m.On("GetAdByID", ctx, mock.Anything).Return(want, nil)

		got, err := usecase.NewGetAdByIDFunc(m)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("GetAdByID", ctx, mock.Anything).Return(domain.Ad{}, adsdao.ErrNotFound)

		_, err := usecase.NewGetAdByIDFunc(m)(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")
		m.On("GetAdByID", ctx, mock.Anything).Return(domain.Ad{}, dbErr)

		_, err := usecase.NewGetAdByIDFunc(m)(ctx, 1)

		assert.ErrorIs(t, err, dbErr)
	})
}
