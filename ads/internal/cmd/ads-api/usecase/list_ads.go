package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const (
	// DefaultListLimit is the number of ads returned when the caller does not ask for a specific number.
	DefaultListLimit = 50
	// MaxListLimit is the largest number of ads a single call returns.
	MaxListLimit = 100
)

type (
	// ListAdsFunc returns a page of the ads matching the filters, newest first.
	ListAdsFunc func(ctx context.Context, req ListAdsRequest) ([]domain.Ad, error)

	// ListAdsRequest describes which ads to return. A zero Limit means DefaultListLimit, and a Limit above MaxListLimit is lowered to it.
	ListAdsRequest struct {
		OwnerID       domain.UserID
		MinPriceCents int64
		MaxPriceCents int64
		Title         string
		Limit         int
		Offset        int
	}

	listAds struct {
		adDAO adsdao.DAO
	}
)

// NewListAdsFunc returns a new ListAdsFunc.
func NewListAdsFunc(adDAO adsdao.DAO) ListAdsFunc {
	return listAds{adDAO: adDAO}.listAds
}

func (uc listAds) listAds(ctx context.Context, req ListAdsRequest) ([]domain.Ad, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}

	limit = min(limit, MaxListLimit)

	ads, err := uc.adDAO.SearchAds(ctx, adsdao.SearchAdsRequest{
		OwnerID:       req.OwnerID,
		MinPriceCents: req.MinPriceCents,
		MaxPriceCents: req.MaxPriceCents,
		Title:         req.Title,
		Limit:         limit,
		Offset:        max(req.Offset, 0),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to search ads: %w", err)
	}

	return ads, nil
}
