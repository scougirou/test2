package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestImportAds(t *testing.T) {
	ctx := context.Background()

	ads := []usecase.ImportedAd{
		{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
		{Title: "scooter", PriceCents: 8000, PhotoURL: "https://example.com/scooter.jpg"},
		{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
	}
	req := usecase.ImportAdsRequest{OwnerID: "user-1", Ads: ads}
	daoReq := func(ad usecase.ImportedAd) adsdao.CreateAdRequest {
		return adsdao.CreateAdRequest{Title: ad.Title, PriceCents: ad.PriceCents, PhotoURL: ad.PhotoURL, OwnerID: "user-1"}
	}

	t.Run("creates every ad in a single transaction", func(t *testing.T) {
		m := mock.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[0])).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[1])).Return(domain.AdID(11), nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[2])).Return(domain.AdID(12), nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11, 12}, got)
	})

	t.Run("does not apply the publication quota", func(t *testing.T) {
		m := mock.New(t)
		many := make([]usecase.ImportedAd, 20)
		m.On("Begin", ctx).Return(nil).Once()
		for i := range many {
			many[i] = usecase.ImportedAd{Title: fmt.Sprintf("ad %d", i), PhotoURL: "https://example.com/p.jpg"}
			m.On("CreateAd", ctx, daoReq(many[i])).Return(domain.AdID(i+1), nil).Once()
		}
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: many})

		require.NoError(t, err)
		assert.Len(t, got, 20)
	})

	t.Run("returns an empty result without opening a transaction when there is nothing to import", func(t *testing.T) {
		m := mock.New(t)

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1"})

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("rolls back and creates nothing when one ad fails", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")
		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[0])).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[1])).Return(domain.AdID(0), boom).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
		m.AssertNotCalled(t, "Commit")
	})

	t.Run("rolls back when the commit fails", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("commit failed")
		m.On("Begin", ctx).Return(nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[0])).Return(domain.AdID(10), nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[1])).Return(domain.AdID(11), nil).Once()
		m.On("CreateAd", ctx, daoReq(ads[2])).Return(domain.AdID(12), nil).Once()
		m.On("Commit").Return(boom).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewImportAdsFunc(m)(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})

	t.Run("returns the error when the transaction cannot be opened", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")
		m.On("Begin", ctx).Return(boom).Once()

		_, err := usecase.NewImportAdsFunc(m)(ctx, req)

		assert.ErrorIs(t, err, boom)
	})

	t.Run("rejects the whole import without touching the database when one ad is invalid", func(t *testing.T) {
		m := mock.New(t)
		invalid := []usecase.ImportedAd{ads[0], {Title: "", PriceCents: 1, PhotoURL: "https://example.com/p.jpg"}, ads[2]}

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: invalid})

		require.ErrorIs(t, err, usecase.ErrInvalidAd)
		assert.Contains(t, err.Error(), "ads[1]")
	})

	t.Run("rejects an import that is too large without touching the database", func(t *testing.T) {
		m := mock.New(t)
		tooMany := make([]usecase.ImportedAd, usecase.MaxImportAds+1)

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: tooMany})

		assert.ErrorIs(t, err, usecase.ErrTooManyAds)
	})
}
