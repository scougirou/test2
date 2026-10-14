package http

import (
	"encoding/json"
	"errors"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func handlerGetAd(getAd usecase.GetAdByIDFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		details, err := getAd(r.Context(), adID)
		if err != nil {
			if errors.Is(err, adsdao.ErrNotFound) {
				rw.WriteHeader(http.StatusNotFound)
			} else {
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(newAdResponse(details.Ad, details.Username))
	}
}
