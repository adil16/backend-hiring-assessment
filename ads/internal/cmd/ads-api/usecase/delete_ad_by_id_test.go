package usecase_test

import (
	"context"
	"errors"
	"testing"

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

		want := domain.Ad{
			ID:     42,
			Title:  "bike",
			Price:  5000,
			UserID: "1",
		}

		mock.InOrder(
			m.On("Begin", ctx).Return(nil).Once(),
			m.On("GetAdByID", ctx, want.ID).Return(want, nil).Once(),
			m.On("DeleteAdByID", ctx, want.ID).Return(nil).Once(),
			m.On("Commit").Return(nil).Once(),
			m.On("Rollback").Return(nil).Once(),
		)

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 42, "1")

		require.NoError(t, err)
	})

	t.Run("error from begin", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")

		m.On("Begin", ctx).Return(dbErr).Once()

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 1, "1")

		require.ErrorIs(t, err, dbErr)
		m.AssertNotCalled(t, "GetAdByID")
		m.AssertNotCalled(t, "DeleteAdByID")
		m.AssertNotCalled(t, "Commit")
		m.AssertNotCalled(t, "Rollback")
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)

		m.On("Begin", ctx).Return(nil).Once()
		m.On("GetAdByID", ctx, domain.AdID(99)).
			Return(domain.Ad{}, adsdao.ErrNotFound).Once()
		m.On("Rollback").Return(nil).Once()

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 99, "99")

		require.ErrorIs(t, err, adsdao.ErrNotFound)
		m.AssertNotCalled(t, "DeleteAdByID")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from get ad", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("GetAdByID", ctx, domain.AdID(1)).
			Return(domain.Ad{}, dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, 1, "1")

		require.ErrorIs(t, err, dbErr)
		m.AssertNotCalled(t, "DeleteAdByID")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error forbidden", func(t *testing.T) {
		m := mockdao.New(t)

		ad := domain.Ad{
			ID:     42,
			UserID: "123",
		}

		m.On("Begin", ctx).Return(nil).Once()
		m.On("GetAdByID", ctx, ad.ID).Return(ad, nil).Once()
		m.On("Rollback").Return(nil).Once()

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, ad.ID, "456")

		require.ErrorIs(t, err, domain.ErrForbidden)
		m.AssertNotCalled(t, "DeleteAdByID")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from delete ad", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")

		ad := domain.Ad{
			ID:     1,
			UserID: "1",
		}

		m.On("Begin", ctx).Return(nil).Once()
		m.On("GetAdByID", ctx, ad.ID).Return(ad, nil).Once()
		m.On("DeleteAdByID", ctx, ad.ID).Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, ad.ID, ad.UserID)

		require.ErrorIs(t, err, dbErr)
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from commit", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("commit failed")

		ad := domain.Ad{
			ID:     1,
			UserID: "1",
		}

		m.On("Begin", ctx).Return(nil).Once()
		m.On("GetAdByID", ctx, ad.ID).Return(ad, nil).Once()
		m.On("DeleteAdByID", ctx, ad.ID).Return(nil).Once()
		m.On("Commit").Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		err := usecase.NewDeleteAdByIDFunc(m)(ctx, ad.ID, ad.UserID)

		require.ErrorIs(t, err, dbErr)
	})
}
