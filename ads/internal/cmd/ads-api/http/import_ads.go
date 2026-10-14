package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type importAdsBody struct {
	Ads []adPayload `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

// handlerImportAds serves POST /ads/import. The import is all or nothing: on any error no ad is created.
func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body importAdsBody
		if err := decodeBody(rw, r, maxImportBodyBytes, &body); err != nil {
			writeBodyError(rw, err)
			return
		}

		ads := make([]usecase.ImportedAd, 0, len(body.Ads))

		for i, item := range body.Ads {
			priceCents, err := item.resolvePrice()
			if err != nil {
				writeError(rw, http.StatusBadRequest, fmt.Sprintf("ads[%d]: %s", i, err))
				return
			}

			ads = append(ads, usecase.ImportedAd{Title: item.Title, PriceCents: priceCents, PhotoURL: item.PhotoURL})
		}

		adIDs, err := importAds(r.Context(), usecase.ImportAdsRequest{
			OwnerID: domain.UserID(middleware.UserIDFromRequest(r)),
			Ads:     ads,
		})
		if err != nil {
			switch {
			case errors.Is(err, usecase.ErrInvalidAd):
				writeError(rw, http.StatusBadRequest, err.Error())
			case errors.Is(err, usecase.ErrTooManyAds):
				writeError(rw, http.StatusRequestEntityTooLarge, err.Error())
			default:
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		ids := make([]int64, 0, len(adIDs))
		for _, id := range adIDs {
			ids = append(ids, int64(id))
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(importAdsResponse{IDs: ids})
	}
}
