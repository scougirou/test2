package usecase

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidAd is returned when the data of an ad to publish is rejected. The wrapping message explains why and is safe to show to the caller.
	ErrInvalidAd = errors.New("invalid ad")
	// ErrTooManyAds is returned when a bulk import holds more ads than allowed.
	ErrTooManyAds = errors.New("too many ads")
	// ErrNotOwner is returned when a user tries to change an ad owned by someone else.
	ErrNotOwner = errors.New("ad belongs to another user")
)

// QuotaExceededError is returned when a user has already published as many ads as the quota allows over the rolling window.
type QuotaExceededError struct {
	// RetryAfter is how long the user has to wait before the quota lets them publish again.
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("publication quota exceeded, retry in %s", e.RetryAfter.Round(time.Second))
}
