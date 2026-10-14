package usecase

import (
	"fmt"
	"strings"
)

func validateAd(title string, priceCents int64, photoURL string) error {
	switch {
	case strings.TrimSpace(title) == "":
		return fmt.Errorf("%w: title must not be empty", ErrInvalidAd)
	case priceCents < 0:
		return fmt.Errorf("%w: price must not be negative", ErrInvalidAd)
	case strings.TrimSpace(photoURL) == "":
		return fmt.Errorf("%w: photo_url must not be empty", ErrInvalidAd)
	case strings.ContainsRune(title, 0) || strings.ContainsRune(photoURL, 0):
		return fmt.Errorf("%w: text fields must not contain NUL characters", ErrInvalidAd)
	}

	return nil
}
