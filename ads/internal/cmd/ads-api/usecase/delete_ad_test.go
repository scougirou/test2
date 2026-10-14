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

func TestDeleteAd(t *testing.T) {
	ctx := context.Background()
	owner := domain.UserID("0a1b2c3d-0000-0000-0000-00000000000a")
	ad := domain.Ad{ID: 42, UserID: owner}
	req := usecase.DeleteAdRequest{AdID: 42, UserID: owner}

	t.Run("deletes the ad of the caller", func(t *testing.T) {
		m := mock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		m.On("DeleteAd", ctx, domain.AdID(42)).Return(nil)

		require.NoError(t, usecase.NewDeleteAdFunc(m)(ctx, req))
	})

	t.Run("recognizes the owner whatever the case of the identifier", func(t *testing.T) {
		m := mock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		m.On("DeleteAd", ctx, domain.AdID(42)).Return(nil)

		err := usecase.NewDeleteAdFunc(m)(ctx, usecase.DeleteAdRequest{AdID: 42, UserID: "0A1B2C3D-0000-0000-0000-00000000000A"})

		require.NoError(t, err)
	})

	t.Run("refuses to delete the ad of someone else", func(t *testing.T) {
		m := mock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)

		err := usecase.NewDeleteAdFunc(m)(ctx, usecase.DeleteAdRequest{AdID: 42, UserID: "00000000-0000-0000-0000-000000000002"})

		assert.ErrorIs(t, err, usecase.ErrNotOwner)
	})

	t.Run("returns ErrNotFound when the ad does not exist", func(t *testing.T) {
		m := mock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{}, adsdao.ErrNotFound)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("returns ErrNotFound when the ad is deleted in the meantime", func(t *testing.T) {
		m := mock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		m.On("DeleteAd", ctx, domain.AdID(42)).Return(adsdao.ErrNotFound)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("wraps the error of the lookup", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(domain.Ad{}, boom)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, boom)
	})

	t.Run("wraps the error of the deletion", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		m.On("DeleteAd", ctx, domain.AdID(42)).Return(boom)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, adsdao.ErrNotFound)
	})
}
