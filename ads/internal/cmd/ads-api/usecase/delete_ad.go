package usecase

import (
	"context"
	"fmt"
	"strings"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// DeleteAdFunc deletes an ad. It fails with dao.ErrNotFound when no ad matches the identifier and with ErrNotOwner when the ad belongs to someone else.
	DeleteAdFunc func(ctx context.Context, req DeleteAdRequest) error

	// DeleteAdRequest describes the ad to delete. UserID is the authenticated user: only the owner of an ad can delete it.
	DeleteAdRequest struct {
		AdID   domain.AdID
		UserID domain.UserID
	}

	deleteAd struct {
		adDAO adsdao.DAO
	}
)

// NewDeleteAdFunc returns a DeleteAdFunc backed by the given DAO.
func NewDeleteAdFunc(adDAO adsdao.DAO) DeleteAdFunc {
	return deleteAd{adDAO: adDAO}.deleteAd
}

func (uc deleteAd) deleteAd(ctx context.Context, req DeleteAdRequest) error {
	ad, err := uc.adDAO.GetAdByID(ctx, req.AdID)
	if err != nil {
		return fmt.Errorf("unable to get ad %s: %w", req.AdID, err)
	}

	// The owner of an ad never changes, so there is no need to check and delete atomically.
	// The comparison ignores case because the database hands back UUIDs in lower case whatever the caller sent.
	if !strings.EqualFold(ad.UserID.String(), req.UserID.String()) {
		return ErrNotOwner
	}

	if err := uc.adDAO.DeleteAd(ctx, req.AdID); err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", req.AdID, err)
	}

	return nil
}
