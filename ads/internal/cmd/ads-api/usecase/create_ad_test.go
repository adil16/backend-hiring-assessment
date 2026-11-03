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

func TestCreateAd(t *testing.T) {
	ctx := context.Background()

	ucReq := usecase.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	daoReq := adsdao.CreateAdRequest{
		Title:      ucReq.Title,
		PriceCents: ucReq.PriceCents,
		PhotoURL:   ucReq.PhotoURL,
		OwnerID:    ucReq.OwnerID,
	}

	t.Run("nominal with 4 recent publications", func(t *testing.T) {
		m := mockdao.New(t)

		mock.InOrder(
			m.On("Begin", ctx).Return(nil).Once(),
			m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(nil).Once(),
			m.On("CountRecentPublications", ctx, ucReq.OwnerID).Return(4, nil).Once(),
			m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once(),
			m.On("RecordPublication", ctx, ucReq.OwnerID).Return(nil).Once(),
			m.On("Commit").Return(nil).Once(),
		)

		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("quota exceeded with 5 recent publications", func(t *testing.T) {
		m := mockdao.New(t)

		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(nil).Once()
		m.On("CountRecentPublications", ctx, ucReq.OwnerID).Return(5, nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, usecase.ErrQuotaExceeded)
		assert.Zero(t, got)

		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "RecordPublication")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from begin", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")

		m.On("Begin", ctx).Return(dbErr).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Zero(t, got)

		m.AssertNotCalled(t, "LockUserPublications")
		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "Commit")
		m.AssertNotCalled(t, "Rollback")
	})

	t.Run("error from lock", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("lock failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Zero(t, got)

		m.AssertNotCalled(t, "CountRecentPublications")
		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from count", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("count failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(nil).Once()
		m.On("CountRecentPublications", ctx, ucReq.OwnerID).Return(0, dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Zero(t, got)

		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "RecordPublication")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("insert failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(nil).Once()
		m.On("CountRecentPublications", ctx, ucReq.OwnerID).Return(2, nil).Once()
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(0), dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Zero(t, got)

		m.AssertNotCalled(t, "RecordPublication")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from record publication", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("record publication failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(nil).Once()
		m.On("CountRecentPublications", ctx, ucReq.OwnerID).Return(2, nil).Once()
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
		m.On("RecordPublication", ctx, ucReq.OwnerID).Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Zero(t, got)

		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from commit", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("commit failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockUserPublications", ctx, ucReq.OwnerID).Return(nil).Once()
		m.On("CountRecentPublications", ctx, ucReq.OwnerID).Return(2, nil).Once()
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
		m.On("RecordPublication", ctx, ucReq.OwnerID).Return(nil).Once()
		m.On("Commit").Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Zero(t, got)
	})
}
