package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	ownerID := domain.UserID("user-1")

	ucReq := usecase.ImportAdsRequest{
		OwnerID: ownerID,
		Ads: []usecase.CreateAdRequest{
			{
				Title:      "bike",
				PriceCents: 5000,
				PhotoURL:   "https://example.com/bike.jpg",
				OwnerID:    ownerID,
			},
			{
				Title:      "scooter",
				PriceCents: 8000,
				PhotoURL:   "https://example.com/scooter.jpg",
				OwnerID:    ownerID,
			},
			{
				Title:      "car",
				PriceCents: 200000,
				PhotoURL:   "https://example.com/car.jpg",
				OwnerID:    ownerID,
			},
		},
	}

	daoReqs := []adsdao.CreateAdRequest{
		{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    ownerID,
		},
		{
			Title:      "scooter",
			PriceCents: 8000,
			PhotoURL:   "https://example.com/scooter.jpg",
			OwnerID:    ownerID,
		},
		{
			Title:      "car",
			PriceCents: 200000,
			PhotoURL:   "https://example.com/car.jpg",
			OwnerID:    ownerID,
		},
	}

	ids := []domain.AdID{10, 11, 12}

	// Request with an idempotency key.
	idempotentReq := ucReq
	idempotentReq.IdempotencyKey = "import-123"

	hash := importAdsHash(t, ucReq.Ads)

	reserveReq := adsdao.ReserveImportRequest{
		UserID:         ownerID,
		IdempotencyKey: idempotentReq.IdempotencyKey,
		RequestHash:    hash,
	}

	t.Run("nominal without idempotency key", func(t *testing.T) {
		m := mockdao.New(t)

		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(ids[0], nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).Return(ids[1], nil).Once()
		m.On("CreateAd", ctx, daoReqs[2]).Return(ids[2], nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, ids, got)

		m.AssertNotCalled(t, "ReserveImport")
		m.AssertNotCalled(t, "CompleteImport")
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

		m.AssertNotCalled(t, "CreateAd", ctx, daoReqs[1])
		m.AssertNotCalled(t, "CreateAd", ctx, daoReqs[2])
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from second create ad", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("insert failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(ids[0], nil).Once()
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
		m.On("CreateAd", ctx, daoReqs[0]).Return(ids[0], nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).Return(ids[1], nil).Once()
		m.On("CreateAd", ctx, daoReqs[2]).Return(ids[2], nil).Once()
		m.On("Commit").Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)
	})

	t.Run("empty import without key does not start transaction", func(t *testing.T) {
		m := mockdao.New(t)

		got, err := usecase.NewImportAdsFunc(m)(
			ctx,
			usecase.ImportAdsRequest{
				OwnerID: ownerID,
				Ads:     []usecase.CreateAdRequest{},
			},
		)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{}, got)

		m.AssertNotCalled(t, "Begin")
		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "Commit")
		m.AssertNotCalled(t, "Rollback")
	})

	t.Run("creates and records a new idempotent import", func(t *testing.T) {
		m := mockdao.New(t)

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, reserveReq).
			Return((*adsdao.ImportResult)(nil), nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(ids[0], nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).Return(ids[1], nil).Once()
		m.On("CreateAd", ctx, daoReqs[2]).Return(ids[2], nil).Once()
		m.On("CompleteImport", ctx, ownerID, "import-123", ids).
			Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, idempotentReq)

		require.NoError(t, err)
		assert.Equal(t, ids, got)
	})

	t.Run("returns existing IDs when retry has same hash", func(t *testing.T) {
		m := mockdao.New(t)

		existing := &adsdao.ImportResult{
			RequestHash: hash,
			AdIDs:       ids,
		}

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, reserveReq).
			Return(existing, nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, idempotentReq)

		require.NoError(t, err)
		assert.Equal(t, ids, got)

		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "CompleteImport")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("returns conflict when retry has different hash", func(t *testing.T) {
		m := mockdao.New(t)

		existing := &adsdao.ImportResult{
			RequestHash: "different-hash",
			AdIDs:       ids,
		}

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, reserveReq).
			Return(existing, nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, idempotentReq)

		require.ErrorIs(t, err, usecase.ErrIdempotencyConflict)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "CompleteImport")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from reserve import", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("unable to reserve key")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, reserveReq).
			Return((*adsdao.ImportResult)(nil), dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, idempotentReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "CreateAd")
		m.AssertNotCalled(t, "CompleteImport")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from create ad with idempotency key", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("insert failed")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, reserveReq).
			Return((*adsdao.ImportResult)(nil), nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(ids[0], nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).
			Return(domain.AdID(0), dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, idempotentReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "CreateAd", ctx, daoReqs[2])
		m.AssertNotCalled(t, "CompleteImport")
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("error from complete import", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("unable to save import result")

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, reserveReq).
			Return((*adsdao.ImportResult)(nil), nil).Once()
		m.On("CreateAd", ctx, daoReqs[0]).Return(ids[0], nil).Once()
		m.On("CreateAd", ctx, daoReqs[1]).Return(ids[1], nil).Once()
		m.On("CreateAd", ctx, daoReqs[2]).Return(ids[2], nil).Once()
		m.On("CompleteImport", ctx, ownerID, "import-123", ids).
			Return(dbErr).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, idempotentReq)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)

		m.AssertNotCalled(t, "Commit")
	})

	t.Run("empty import with key is recorded", func(t *testing.T) {
		m := mockdao.New(t)

		req := usecase.ImportAdsRequest{
			OwnerID:        ownerID,
			IdempotencyKey: "empty-import-123",
			Ads:            []usecase.CreateAdRequest{},
		}

		emptyHash := importAdsHash(t, req.Ads)
		emptyIDs := []domain.AdID{}

		m.On("Begin", ctx).Return(nil).Once()
		m.On("ReserveImport", ctx, adsdao.ReserveImportRequest{
			UserID:         ownerID,
			IdempotencyKey: req.IdempotencyKey,
			RequestHash:    emptyHash,
		}).Return((*adsdao.ImportResult)(nil), nil).Once()
		m.On("CompleteImport", ctx, ownerID, req.IdempotencyKey, emptyIDs).
			Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, emptyIDs, got)

		m.AssertNotCalled(t, "CreateAd")
	})
}

// importAdsHash computes the expected hash for a given import payload.
func importAdsHash(t *testing.T, ads []usecase.CreateAdRequest) string {
	t.Helper()

	if ads == nil {
		ads = []usecase.CreateAdRequest{}
	}

	payload, err := json.Marshal(ads)
	require.NoError(t, err)

	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}
