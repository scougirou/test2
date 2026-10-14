package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// MaxImportAds is the largest number of ads a single import can hold.
const MaxImportAds = 1000

type (
	// ImportAdsFunc creates all the given ads, or none of them: a failed import leaves nothing behind, so it can safely be run again. It returns the identifiers in the order of the request.
	// It fails with ErrInvalidAd when one of the ads is rejected and with ErrTooManyAds when the import is too large. The publication quota does not apply.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// ImportAdsRequest describes the ads to import. OwnerID is the authenticated user and owns every ad.
	ImportAdsRequest struct {
		OwnerID domain.UserID
		Ads     []ImportedAd
	}

	// ImportedAd is one ad of an import.
	ImportedAd struct {
		Title      string
		PriceCents int64
		PhotoURL   string
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
	if len(req.Ads) > MaxImportAds {
		return nil, fmt.Errorf("%w: an import holds at most %d ads", ErrTooManyAds, MaxImportAds)
	}

	// Validating everything first rejects a bad import without touching the database.
	for i, ad := range req.Ads {
		if err := validateAd(ad.Title, ad.PriceCents, ad.PhotoURL); err != nil {
			return nil, fmt.Errorf("ads[%d]: %w", i, err)
		}
	}

	ids := make([]domain.AdID, 0, len(req.Ads))
	if len(req.Ads) == 0 {
		return ids, nil
	}

	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, ad := range req.Ads {
		id, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
			Title:      ad.Title,
			PriceCents: ad.PriceCents,
			PhotoURL:   ad.PhotoURL,
			OwnerID:    req.OwnerID,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to import ads[%d]: %w", i, err)
		}

		ids = append(ids, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}

	return ids, nil
}
