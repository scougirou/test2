package http

import (
	"errors"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// adPayload is an ad as sent by a client to be published.
type adPayload struct {
	Title    string `json:"title"`
	PhotoURL string `json:"photo_url"`
	// Price is the price in euro cents.
	Price *int64 `json:"price"`
	// PriceCents is the former name of Price. It is still accepted so that clients can migrate at their own pace.
	//
	// Deprecated: use Price.
	PriceCents *int64 `json:"price_cents"`
}

// resolvePrice returns the price of the ad, whichever of the current and the former field name was used.
func (p adPayload) resolvePrice() (int64, error) {
	switch {
	case p.Price == nil && p.PriceCents == nil:
		return 0, errors.New("price is required")
	case p.Price != nil && p.PriceCents != nil && *p.Price != *p.PriceCents:
		return 0, errors.New("price and price_cents are both set to different values")
	case p.Price != nil:
		return *p.Price, nil
	default:
		return *p.PriceCents, nil
	}
}

// adResponse is an ad as returned to clients.
type adResponse struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	// Price is the price in euro cents.
	Price int64 `json:"price"`
	// PriceCents duplicates Price for the clients that have not migrated yet.
	//
	// Deprecated: use Price.
	PriceCents int64     `json:"price_cents"`
	PhotoURL   string    `json:"photo_url"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func newAdResponse(a domain.Ad, username string) adResponse {
	return adResponse{
		ID:         int64(a.ID),
		Title:      a.Title,
		Price:      a.Price,
		PriceCents: a.Price,
		PhotoURL:   a.PhotoURL,
		UserID:     a.UserID.String(),
		Username:   username,
		CreatedAt:  a.CreatedAt.UTC(),
	}
}
