package mock_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var ctx = context.Background()

func TestMock_SearchAds(t *testing.T) {
	t.Run("returns configured ads", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.SearchAdsRequest{Title: "bike"}
		want := []domain.Ad{{ID: 1, Title: "bike", Price: 5000}}

		m.On("SearchAds", ctx, req).Return(want, nil)

		got, err := m.SearchAds(ctx, req)

		require.Error(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns nil slice on error", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.SearchAdsRequest{Title: "bike"}
		boom := errors.New("db down")

		m.On("SearchAds", ctx, req).Return(nil, boom)

		got, err := m.SearchAds(ctx, req)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})
}

func TestMock_GetAdByID(t *testing.T) {
	t.Run("returns configured ad", func(t *testing.T) {
		m := mock.New(t)
		want := domain.Ad{ID: 42, Title: "surfboard", Price: 15000}

		m.On("GetAdByID", ctx, domain.AdID(42)).Return(want, nil)

		got, err := m.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns ErrNotFound", func(t *testing.T) {
		m := mock.New(t)

		m.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, adsdao.ErrNotFound)

		_, err := m.GetAdByID(ctx, 99)

		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})
}

func TestMock_CreateAd(t *testing.T) {
	t.Run("returns generated ID", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.CreateAdRequest{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "user-1",
		}

		m.On("CreateAd", ctx, req).Return(domain.AdID(7), nil)

		got, err := m.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("returns ErrUnknownOwner when owner does not exist", func(t *testing.T) {
		m := mock.New(t)
		req := adsdao.CreateAdRequest{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "ghost",
		}

		m.On("CreateAd", ctx, req).Return(domain.AdID(0), adsdao.ErrUnknownOwner)

		_, err := m.CreateAd(ctx, req)

		assert.ErrorIs(t, err, adsdao.ErrUnknownOwner)
	})
}

func TestMock_Begin(t *testing.T) {
	t.Run("returns the mock itself as transaction", func(t *testing.T) {
		m := mock.New(t)

		m.On("Begin", ctx).Return(nil)

		tx, err := m.Begin(ctx)

		require.NoError(t, err)
		assert.Same(t, m, tx)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("Begin", ctx).Return(boom)

		tx, err := m.Begin(ctx)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
	})
}

func TestMock_Commit(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("Commit").Return(nil)

		require.NoError(t, m.Commit())
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("commit failed")

		m.On("Commit").Return(boom)

		assert.ErrorIs(t, m.Commit(), boom)
	})
}

func TestMock_Rollback(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("Rollback").Return(nil)

		require.NoError(t, m.Rollback())
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("rollback failed")

		m.On("Rollback").Return(boom)

		assert.ErrorIs(t, m.Rollback(), boom)
	})
}
