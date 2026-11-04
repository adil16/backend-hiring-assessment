package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used with a different import")

type (
	// ImportAdsFunc creates multiple ads atomically.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// ImportAdsRequest describes the ads to import.
	ImportAdsRequest struct {
		Ads            []CreateAdRequest
		OwnerID        domain.UserID
		IdempotencyKey string
	}

	importAds struct {
		adDAO adsdao.DAO
	}
)

// NewImportAdsFunc returns an ImportAdsFunc backed by the given DAO.
func NewImportAdsFunc(adDAO adsdao.DAO) ImportAdsFunc {
	return importAds{adDAO: adDAO}.importAds
}

func (uc importAds) importAds(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error) {
	// An empty import without an idempotency key needs no transaction.
	if len(req.Ads) == 0 && req.IdempotencyKey == "" {
		return []domain.AdID{}, nil
	}

	var requestHash string

	if req.IdempotencyKey != "" {
		// Normalize nil and empty slices to the same payload.
		ads := req.Ads
		if ads == nil {
			ads = []CreateAdRequest{}
		}

		payload, err := json.Marshal(ads)
		if err != nil {
			return nil, fmt.Errorf("unable to serialize import: %w", err)
		}

		hash := sha256.Sum256(payload)
		requestHash = hex.EncodeToString(hash[:])
	}

	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	if req.IdempotencyKey != "" {
		existing, err := tx.ReserveImport(ctx, adsdao.ReserveImportRequest{
			UserID:         req.OwnerID,
			IdempotencyKey: req.IdempotencyKey,
			RequestHash:    requestHash,
		})
		if err != nil {
			return nil, fmt.Errorf("reserve import: %w", err)
		}

		if existing != nil {
			if existing.RequestHash != requestHash {
				return nil, ErrIdempotencyConflict
			}

			// Already imported: return the original IDs.
			return existing.AdIDs, nil
		}
	}

	ids := make([]domain.AdID, 0, len(req.Ads))

	for i, ad := range req.Ads {
		id, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
			Title:      ad.Title,
			PriceCents: ad.PriceCents,
			PhotoURL:   ad.PhotoURL,
			OwnerID:    ad.OwnerID,
		})
		if err != nil {
			return nil, fmt.Errorf("create ad at index %d: %w", i, err)
		}

		ids = append(ids, id)
	}

	if req.IdempotencyKey != "" {
		if err := tx.CompleteImport(ctx, req.OwnerID, req.IdempotencyKey, ids); err != nil {
			return nil, fmt.Errorf("complete import: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return ids, nil
}
