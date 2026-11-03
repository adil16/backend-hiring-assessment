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

		require.NoError(t, err)
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

func TestMock_DeleteAdByID(t *testing.T) {
	t.Run("returns nil (no error)", func(t *testing.T) {
		m := mock.New(t)

		m.On("DeleteAdByID", ctx, domain.AdID(42)).Return(nil)

		err := m.DeleteAdByID(ctx, 42)

		require.NoError(t, err)
	})

	t.Run("returns ErrNotFound", func(t *testing.T) {
		m := mock.New(t)

		m.On("DeleteAdByID", ctx, domain.AdID(99)).Return(adsdao.ErrNotFound)

		err := m.DeleteAdByID(ctx, 99)

		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})
}

func TestMock_ReserveImport(t *testing.T) {
	req := adsdao.ReserveImportRequest{
		UserID:         "user-1",
		IdempotencyKey: "import-123",
		RequestHash:    "hash-123",
	}

	t.Run("returns nil when key is reserved", func(t *testing.T) {
		m := mock.New(t)

		m.On("ReserveImport", ctx, req).
			Return((*adsdao.ImportResult)(nil), nil)

		got, err := m.ReserveImport(ctx, req)

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns existing import", func(t *testing.T) {
		m := mock.New(t)

		want := &adsdao.ImportResult{
			RequestHash: "hash-123",
			AdIDs:       []domain.AdID{10, 11, 12},
		}

		m.On("ReserveImport", ctx, req).Return(want, nil)

		got, err := m.ReserveImport(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("ReserveImport", ctx, req).
			Return((*adsdao.ImportResult)(nil), boom)

		got, err := m.ReserveImport(ctx, req)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})
}

func TestMock_CompleteImport(t *testing.T) {
	userID := domain.UserID("user-1")
	key := "import-123"
	ids := []domain.AdID{10, 11, 12}

	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("CompleteImport", ctx, userID, key, ids).
			Return(nil)

		err := m.CompleteImport(ctx, userID, key, ids)

		require.NoError(t, err)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("CompleteImport", ctx, userID, key, ids).
			Return(boom)

		err := m.CompleteImport(ctx, userID, key, ids)

		assert.ErrorIs(t, err, boom)
	})
}

func TestMock_LockUserPublications(t *testing.T) {
	userID := domain.UserID("user-1")

	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("LockUserPublications", ctx, userID).Return(nil)

		err := m.LockUserPublications(ctx, userID)

		require.NoError(t, err)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("lock failed")

		m.On("LockUserPublications", ctx, userID).Return(boom)

		err := m.LockUserPublications(ctx, userID)

		assert.ErrorIs(t, err, boom)
	})
}

func TestMock_CountRecentPublications(t *testing.T) {
	userID := domain.UserID("user-1")

	t.Run("returns configured count", func(t *testing.T) {
		m := mock.New(t)

		m.On("CountRecentPublications", ctx, userID).Return(4, nil)

		got, err := m.CountRecentPublications(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, 4, got)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("CountRecentPublications", ctx, userID).Return(0, boom)

		got, err := m.CountRecentPublications(ctx, userID)

		assert.ErrorIs(t, err, boom)
		assert.Zero(t, got)
	})
}

func TestMock_RecordPublication(t *testing.T) {
	userID := domain.UserID("user-1")

	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("RecordPublication", ctx, userID).Return(nil)

		err := m.RecordPublication(ctx, userID)

		require.NoError(t, err)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("RecordPublication", ctx, userID).Return(boom)

		err := m.RecordPublication(ctx, userID)

		assert.ErrorIs(t, err, boom)
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
