package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

type listAdsResponse struct {
	Ads []adResponse `json:"ads"`
}

func handlerListAds(listAds usecase.ListAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		req, err := listAdsRequestFromQuery(r.URL.Query())
		if err != nil {
			writeError(rw, http.StatusBadRequest, err.Error())
			return
		}

		ads, err := listAds(r.Context(), req)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		items := make([]adResponse, 0, len(ads))
		for _, a := range ads {
			items = append(items, newAdResponse(a, ""))
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(listAdsResponse{Ads: items})
	}
}

func listAdsRequestFromQuery(q url.Values) (usecase.ListAdsRequest, error) {
	req := usecase.ListAdsRequest{
		Title:   q.Get("title"),
		OwnerID: domain.UserID(q.Get("user_id")),
	}

	var err error

	if req.MinPriceCents, err = priceParam(q, "min_price", "min_price_cents"); err != nil {
		return usecase.ListAdsRequest{}, err
	}

	if req.MaxPriceCents, err = priceParam(q, "max_price", "max_price_cents"); err != nil {
		return usecase.ListAdsRequest{}, err
	}

	if req.Limit, err = countParam(q, "limit"); err != nil {
		return usecase.ListAdsRequest{}, err
	}

	if req.Offset, err = countParam(q, "offset"); err != nil {
		return usecase.ListAdsRequest{}, err
	}

	return req, nil
}

// priceParam reads a price filter, whichever of the current and the former parameter name was used. Absent means 0.
func priceParam(q url.Values, name, legacyName string) (int64, error) {
	value, err := nonNegativeInt(q, name)
	if err != nil {
		return 0, err
	}

	legacyValue, err := nonNegativeInt(q, legacyName)
	if err != nil {
		return 0, err
	}

	if q.Has(name) && q.Has(legacyName) && value != legacyValue {
		return 0, fmt.Errorf("%s and %s are both set to different values", name, legacyName)
	}

	if q.Has(name) {
		return value, nil
	}

	return legacyValue, nil
}

func countParam(q url.Values, name string) (int, error) {
	value, err := nonNegativeInt(q, name)
	if err != nil {
		return 0, err
	}

	return int(value), nil
}

// nonNegativeInt reads an optional query parameter. Absent or empty means 0.
func nonNegativeInt(q url.Values, name string) (int64, error) {
	raw := q.Get(name)
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}

	if value < 0 {
		return 0, errors.New(name + " must not be negative")
	}

	return value, nil
}
