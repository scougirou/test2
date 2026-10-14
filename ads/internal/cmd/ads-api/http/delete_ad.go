package http

import (
	"errors"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

// handlerDeleteAd serves DELETE /ads/{adID}.
func handlerDeleteAd(deleteAd usecase.DeleteAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		err = deleteAd(r.Context(), usecase.DeleteAdRequest{
			AdID:   adID,
			UserID: middleware.UserIDFromRequest(r),
		})

		switch {
		case err == nil:
			rw.WriteHeader(http.StatusNoContent)
		case errors.Is(err, adsdao.ErrNotFound):
			rw.WriteHeader(http.StatusNotFound)
		case errors.Is(err, usecase.ErrNotOwner):
			rw.WriteHeader(http.StatusForbidden)
		default:
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}
}
