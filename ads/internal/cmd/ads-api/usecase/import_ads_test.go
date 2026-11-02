package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestImportAds(t *testing.T) {
	ctx := context.Background()

	ucReq := usecase.ImportAdsRequest{
		Ads: []usecase.CreateAdRequest{
			{
				Title:      "bike",
				PriceCents: 5000,
				PhotoURL:   "https://example.com/bike.jpg",
				OwnerID:    "user-1",
			},
			{
				Title:      "scooter",
				PriceCents: 8000,
				PhotoURL:   "https://example.com/scooter.jpg",
				OwnerID:    "user-1",
			},
			{
				Title:      "car",
				PriceCents: 200000,
				PhotoURL:   "https://example.com/car.jpg",
				OwnerID:    "user-1",
			},
		},
	}

	daoReqs := []adsdao.CreateAdRequest{
		{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "user-1",
		},
		{
			Title:      "scooter",
			PriceCents: 8000,
			PhotoURL:   "https://example.com/scooter.jpg",
			OwnerID:    "user-1",
		},
		{
			Title:      "car",
			PriceCents: 200000,
			PhotoURL:   "https://example.com/car.jpg",
			OwnerID:    "user-1",
		},
	}

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)

		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).Return(domain.AdID(11), nil).Once()
		m.On("CreateAd", ctx, daoReqs[2]).Return(domain.AdID(12), nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11, 12}, got)
	})

	t.Run("error from begin", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")

		m.On("Begin", ctx).Return(dbErr).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "Commit")
		m.AssertNotCalled(t, "Rollback")
	})

	t.Run("error from first create ad", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("insert failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).
			Return(domain.AdID(0), dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "Commit")
		m.AssertNotCalled(t, "CreateAd", ctx, daoReqs[1])
		m.AssertNotCalled(t, "CreateAd", ctx, daoReqs[2])
	})

	t.Run("error from second create ad", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("insert failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).
			Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).
			Return(domain.AdID(0), dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "CreateAd", ctx, daoReqs[2])
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from commit", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("commit failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).Return(domain.AdID(11), nil).Once()
		m.On("CreateAd", ctx, daoReqs[2]).Return(domain.AdID(12), nil).Once()
		m.On("Commit").Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)
	})

	t.Run("empty import does not start transaction", func(t *testing.T) {
		m := mockdao.New(t)

		got, err := usecase.NewImportAdsFunc(m)(
			ctx,
			usecase.ImportAdsRequest{Ads: []usecase.CreateAdRequest{}},
		)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{}, got)

		m.AssertNotCalled(t, "Begin")
		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "Commit")
		m.AssertNotCalled(t, "Rollback")
	})
}
