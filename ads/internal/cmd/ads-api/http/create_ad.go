package http

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type createAdResponse struct {
	ID int64 `json:"id"`
}

// handlerCreateAd serves POST /ads.
func handlerCreateAd(createAd usecase.CreateAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body adPayload
		if err := decodeBody(rw, r, maxBodyBytes, &body); err != nil {
			writeBodyError(rw, err)
			return
		}

		priceCents, err := body.resolvePrice()
		if err != nil {
			writeError(rw, http.StatusBadRequest, err.Error())
			return
		}

		adID, err := createAd(r.Context(), usecase.CreateAdRequest{
			Title:      body.Title,
			PriceCents: priceCents,
			PhotoURL:   body.PhotoURL,
			OwnerID:    middleware.UserIDFromRequest(r),
		})
		if err != nil {
			var quotaErr *usecase.QuotaExceededError

			switch {
			case errors.Is(err, usecase.ErrInvalidAd):
				writeError(rw, http.StatusBadRequest, err.Error())
			case errors.As(err, &quotaErr):
				// Retry-After is expressed in whole seconds, rounded up so that a client obeying it is not turned down again.
				rw.Header().Set("Retry-After", strconv.FormatInt(int64(math.Ceil(quotaErr.RetryAfter.Seconds())), 10))
				writeError(rw, http.StatusTooManyRequests, "you have reached the maximum number of ads you can publish per day")
			default:
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(createAdResponse{ID: int64(adID)})
	}
}
