package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestListAds(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		t.Run("returns all matching ads", func(t *testing.T) {
			m := mock.New(t)
			want := []domain.Ad{
				{ID: 1, Title: "bike", Price: 5000, UserID: "user-1"},
				{ID: 2, Title: "scooter", Price: 12000, UserID: "user-2"},
			}
			req := adsdao.SearchAdsRequest{Title: "bike", Limit: usecase.DefaultListLimit}
			m.On("SearchAds", ctx, req).Return(want, nil)

			got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Title: "bike"})

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})

		t.Run("returns empty slice when no ads match", func(t *testing.T) {
			m := mock.New(t)
			m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Title: "ufo", Limit: usecase.DefaultListLimit}).Return(nil, nil)

			got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Title: "ufo"})

			require.NoError(t, err)
			assert.Nil(t, got)
		})

		t.Run("forwards every filter", func(t *testing.T) {
			m := mock.New(t)
			m.On("SearchAds", ctx, adsdao.SearchAdsRequest{
				OwnerID:       "user-1",
				MinPriceCents: 100,
				MaxPriceCents: 900,
				Title:         "bike",
				Limit:         20,
				Offset:        40,
			}).Return(nil, nil)

			_, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{
				OwnerID:       "user-1",
				MinPriceCents: 100,
				MaxPriceCents: 900,
				Title:         "bike",
				Limit:         20,
				Offset:        40,
			})

			require.NoError(t, err)
		})
	})

	t.Run("bounds the page size", func(t *testing.T) {
		for name, tc := range map[string]struct{ asked, used int }{
			"defaults when not asked": {asked: 0, used: usecase.DefaultListLimit},
			"defaults when negative":  {asked: -3, used: usecase.DefaultListLimit},
			"keeps a small page":      {asked: 3, used: 3},
			"keeps the maximum":       {asked: usecase.MaxListLimit, used: usecase.MaxListLimit},
			"lowers an excessive one": {asked: usecase.MaxListLimit + 1, used: usecase.MaxListLimit},
		} {
			t.Run(name, func(t *testing.T) {
				m := mock.New(t)
				m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Limit: tc.used}).Return(nil, nil)

				_, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Limit: tc.asked})

				require.NoError(t, err)
			})
		}
	})

	t.Run("ignores a negative offset", func(t *testing.T) {
		m := mock.New(t)
		m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Limit: usecase.DefaultListLimit}).Return(nil, nil)

		_, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Offset: -5})

		require.NoError(t, err)
	})

	t.Run("wraps DAO errors", func(t *testing.T) {
		m := mock.New(t)
		dbErr := errors.New("connection reset")
		m.On("SearchAds", ctx, adsdao.SearchAdsRequest{Limit: usecase.DefaultListLimit}).Return(nil, dbErr)

		_, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{})

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})
}
