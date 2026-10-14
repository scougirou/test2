package dao

import (
	"context"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// Store reads and writes ads. It is implemented both by the DAO, where every call is its own transaction, and by a Tx opened from it.
type Store interface {
	// CreateAd inserts an ad and returns its generated identifier.
	CreateAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error)
	// GetAdByID returns a single ad, or ErrNotFound.
	GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error)
	// SearchAds returns the ads matching the filters, newest first.
	SearchAds(ctx context.Context, req SearchAdsRequest) ([]domain.Ad, error)
	// DeleteAd removes an ad, or returns ErrNotFound when it does not exist.
	DeleteAd(ctx context.Context, id domain.AdID) error
	// ListPublications returns, oldest first, the instants at which the owner published an ad through the quota-limited path since the given instant.
	ListPublications(ctx context.Context, ownerID domain.UserID, since time.Time) ([]time.Time, error)
	// RecordPublication records that the owner published an ad at the given instant. Publications are kept apart from the ads so that deleting an ad does not give quota back.
	RecordPublication(ctx context.Context, ownerID domain.UserID, at time.Time) error
}

// DAO stores and retrieves ads.
type DAO interface {
	Store
	// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
	Begin(ctx context.Context) (Tx, error)
}

// Tx is a Store scoped to an opened transaction.
type Tx interface {
	Store
	// LockOwner blocks until no other transaction holds the owner's lock, then holds it until this transaction ends. It serializes the transactions that read then write the owner's publications.
	LockOwner(ctx context.Context, ownerID domain.UserID) error
	// Commit applies every change made through the transaction.
	Commit() error
	// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
	Rollback() error
}

// SearchAdsRequest describes an ad search. Zero values mean "no filter on this field". A zero Limit means "no limit".
type SearchAdsRequest struct {
	OwnerID       domain.UserID
	MinPriceCents int64
	MaxPriceCents int64
	Title         string
	Limit         int
	Offset        int
}

// CreateAdRequest carries the data needed to create an ad.
type CreateAdRequest struct {
	Title      string
	PriceCents int64
	PhotoURL   string
	OwnerID    domain.UserID
}

const (
	// ErrNotFound is returned when no ad matches the requested identifier.
	ErrNotFound sentinelError = "ad not found"
	// ErrUnknownOwner is returned when an ad references a user that does not exist.
	ErrUnknownOwner sentinelError = "unknown owner"
)

type sentinelError string

// Error implements the error interface.
func (e sentinelError) Error() string { return string(e) }
