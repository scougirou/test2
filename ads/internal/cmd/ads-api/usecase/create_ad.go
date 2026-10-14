package usecase

import (
	"context"
	"fmt"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// DefaultQuota is the publication quota applied to POST /ads.
var DefaultQuota = Quota{Limit: 5, Window: 24 * time.Hour}

type (
	// CreateAdFunc creates an ad owned by the calling user. It fails with a *QuotaExceededError when the user has used up their quota, and with ErrInvalidAd when the ad is rejected.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create. OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	// Quota caps the number of ads a user can publish over a rolling window. Limit must be positive.
	Quota struct {
		Limit  int
		Window time.Duration
	}

	createAd struct {
		adDAO adsdao.DAO
		quota Quota
		now   func() time.Time
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given DAO that enforces the given quota.
func NewCreateAdFunc(adDAO adsdao.DAO, quota Quota, now func() time.Time) CreateAdFunc {
	return createAd{adDAO: adDAO, quota: quota, now: now}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	if err := validateAd(req.Title, req.PriceCents, req.PhotoURL); err != nil {
		return 0, err
	}

	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Two concurrent requests of the same user would otherwise both count the same publications and both pass the quota.
	if err := tx.LockOwner(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	now := uc.now()

	publications, err := tx.ListPublications(ctx, req.OwnerID, now.Add(-uc.quota.Window))
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if len(publications) >= uc.quota.Limit {
		// The user can publish again once enough publications left the window to go under the limit.
		unlockedBy := publications[len(publications)-uc.quota.Limit]

		return 0, &QuotaExceededError{RetryAfter: unlockedBy.Add(uc.quota.Window).Sub(now)}
	}

	adID, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
		Title:      req.Title,
		PriceCents: req.PriceCents,
		PhotoURL:   req.PhotoURL,
		OwnerID:    req.OwnerID,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.RecordPublication(ctx, req.OwnerID, now); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return adID, nil
}
