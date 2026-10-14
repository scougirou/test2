package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestCreateAd(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-24 * time.Hour)
	quota := usecase.Quota{Limit: 5, Window: 24 * time.Hour}
	clock := func() time.Time { return now }

	ucReq := usecase.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}
	daoReq := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("creates the ad and records the publication in one transaction", func(t *testing.T) {
		m := mock.New(t)
		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
		m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(nil, nil).Once()
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
		m.On("RecordPublication", ctx, domain.UserID("user-1"), now).Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m, quota, clock)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("still creates the ad one publication below the quota", func(t *testing.T) {
		m := mock.New(t)
		publications := []time.Time{now.Add(-5 * time.Hour), now.Add(-4 * time.Hour), now.Add(-3 * time.Hour), now.Add(-time.Hour)}
		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
		m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(publications, nil).Once()
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
		m.On("RecordPublication", ctx, domain.UserID("user-1"), now).Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		got, err := usecase.NewCreateAdFunc(m, quota, clock)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("refuses the ad once the quota is reached and says when to come back", func(t *testing.T) {
		m := mock.New(t)
		publications := []time.Time{
			now.Add(-20 * time.Hour), // leaves the window in 4 hours: the user can publish again then
			now.Add(-15 * time.Hour),
			now.Add(-10 * time.Hour),
			now.Add(-5 * time.Hour),
			now.Add(-time.Hour),
		}
		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
		m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(publications, nil).Once()
		m.On("Rollback").Return(nil).Once()

		_, err := usecase.NewCreateAdFunc(m, quota, clock)(ctx, ucReq)

		var quotaErr *usecase.QuotaExceededError
		require.ErrorAs(t, err, &quotaErr)
		assert.Equal(t, 4*time.Hour, quotaErr.RetryAfter)
	})

	t.Run("rejects an invalid ad without touching the database", func(t *testing.T) {
		for name, mutate := range map[string]func(*usecase.CreateAdRequest){
			"empty title":          func(r *usecase.CreateAdRequest) { r.Title = "  " },
			"negative price":       func(r *usecase.CreateAdRequest) { r.PriceCents = -1 },
			"empty photo url":      func(r *usecase.CreateAdRequest) { r.PhotoURL = "" },
			"NUL in title":         func(r *usecase.CreateAdRequest) { r.Title = "bike\x00" },
			"NUL in photo url":     func(r *usecase.CreateAdRequest) { r.PhotoURL = "https://example.com/\x00" },
			"whitespace photo url": func(r *usecase.CreateAdRequest) { r.PhotoURL = " " },
		} {
			t.Run(name, func(t *testing.T) {
				m := mock.New(t)
				req := ucReq
				mutate(&req)

				_, err := usecase.NewCreateAdFunc(m, quota, clock)(ctx, req)

				assert.ErrorIs(t, err, usecase.ErrInvalidAd)
			})
		}
	})

	t.Run("accepts a free ad", func(t *testing.T) {
		m := mock.New(t)
		free := daoReq
		free.PriceCents = 0
		m.On("Begin", ctx).Return(nil).Once()
		m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
		m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(nil, nil).Once()
		m.On("CreateAd", ctx, free).Return(domain.AdID(8), nil).Once()
		m.On("RecordPublication", ctx, domain.UserID("user-1"), now).Return(nil).Once()
		m.On("Commit").Return(nil).Once()
		m.On("Rollback").Return(nil).Once()

		req := ucReq
		req.PriceCents = 0
		got, err := usecase.NewCreateAdFunc(m, quota, clock)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(8), got)
	})

	boom := errors.New("db down")

	t.Run("wraps the error of the step that failed and rolls back", func(t *testing.T) {
		steps := map[string]func(m *mock.Mock){
			"begin": func(m *mock.Mock) {
				m.On("Begin", ctx).Return(boom).Once()
			},
			"lock": func(m *mock.Mock) {
				m.On("Begin", ctx).Return(nil).Once()
				m.On("LockOwner", ctx, domain.UserID("user-1")).Return(boom).Once()
				m.On("Rollback").Return(nil).Once()
			},
			"list publications": func(m *mock.Mock) {
				m.On("Begin", ctx).Return(nil).Once()
				m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
				m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(nil, boom).Once()
				m.On("Rollback").Return(nil).Once()
			},
			"create ad": func(m *mock.Mock) {
				m.On("Begin", ctx).Return(nil).Once()
				m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
				m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(nil, nil).Once()
				m.On("CreateAd", ctx, daoReq).Return(domain.AdID(0), boom).Once()
				m.On("Rollback").Return(nil).Once()
			},
			"record publication": func(m *mock.Mock) {
				m.On("Begin", ctx).Return(nil).Once()
				m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
				m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(nil, nil).Once()
				m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
				m.On("RecordPublication", ctx, domain.UserID("user-1"), now).Return(boom).Once()
				m.On("Rollback").Return(nil).Once()
			},
			"commit": func(m *mock.Mock) {
				m.On("Begin", ctx).Return(nil).Once()
				m.On("LockOwner", ctx, domain.UserID("user-1")).Return(nil).Once()
				m.On("ListPublications", ctx, domain.UserID("user-1"), windowStart).Return(nil, nil).Once()
				m.On("CreateAd", ctx, daoReq).Return(domain.AdID(7), nil).Once()
				m.On("RecordPublication", ctx, domain.UserID("user-1"), now).Return(nil).Once()
				m.On("Commit").Return(boom).Once()
				m.On("Rollback").Return(nil).Once()
			},
		}

		for name, setup := range steps {
			t.Run(name, func(t *testing.T) {
				m := mock.New(t)
				setup(m)

				got, err := usecase.NewCreateAdFunc(m, quota, clock)(ctx, ucReq)

				require.Error(t, err)
				assert.ErrorIs(t, err, boom)
				assert.Zero(t, got)
			})
		}
	})
}
