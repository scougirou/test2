package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

type (
	// GetAdByIDFunc returns a single ad along with the nickname of its owner. It fails with dao.ErrNotFound when no ad matches the identifier.
	// The nickname is an enrichment: when the user service cannot provide it, the ad is still returned, with an empty Username.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (AdDetails, error)

	// AdDetails is an ad and the nickname of its owner.
	AdDetails struct {
		Ad       domain.Ad
		Username string
	}

	getAdByID struct {
		adDAO adsdao.DAO
		users usersclient.UserClient
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc backed by the given DAO and user service client.
func NewGetAdByIDFunc(adDAO adsdao.DAO, users usersclient.UserClient) GetAdByIDFunc {
	return getAdByID{adDAO: adDAO, users: users}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (AdDetails, error) {
	found, err := uc.adDAO.GetAdByID(ctx, id)
	if err != nil {
		return AdDetails{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	return AdDetails{Ad: found, Username: uc.username(ctx, found.UserID)}, nil
}

// username never fails: ads are read far more than anything else and must stay available when the user service is not.
func (uc getAdByID) username(ctx context.Context, userID domain.UserID) string {
	user, err := uc.users.GetUserByID(ctx, userID)
	if err != nil {
		if !errors.Is(err, usersapi.ErrNotFound) {
			slog.WarnContext(ctx, "unable to get the owner of an ad", "user_id", userID, "error", err)
		}

		return ""
	}

	return user.Nickname
}
