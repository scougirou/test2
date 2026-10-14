package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestRouter(t *testing.T) {
	calls := map[string]int{}
	router := NewRouter(Usecases{
		ListAds: func(context.Context, usecase.ListAdsRequest) ([]domain.Ad, error) {
			calls["list"]++
			return nil, nil
		},
		GetAd: func(_ context.Context, id domain.AdID) (usecase.AdDetails, error) {
			calls["get"]++
			return usecase.AdDetails{Ad: domain.Ad{ID: id}}, nil
		},
		CreateAd: func(context.Context, usecase.CreateAdRequest) (domain.AdID, error) {
			calls["create"]++
			return 1, nil
		},
		DeleteAd: func(context.Context, usecase.DeleteAdRequest) error {
			calls["delete"]++
			return nil
		},
		ImportAds: func(context.Context, usecase.ImportAdsRequest) ([]domain.AdID, error) {
			calls["import"]++
			return []domain.AdID{}, nil
		},
	})

	const body = `{"title":"bike","price":1,"photo_url":"u"}`

	for name, tc := range map[string]struct {
		method, path, body, auth string
		wantStatus               int
		wantCall                 string
	}{
		"health is public":                 {method: "GET", path: "/health", wantStatus: http.StatusOK},
		"list is public":                   {method: "GET", path: "/ads", wantStatus: http.StatusOK, wantCall: "list"},
		"get is public":                    {method: "GET", path: "/ads/1", wantStatus: http.StatusOK, wantCall: "get"},
		"create requires auth":             {method: "POST", path: "/ads", body: body, wantStatus: http.StatusUnauthorized},
		"create":                           {method: "POST", path: "/ads", body: body, auth: "user-1", wantStatus: http.StatusCreated, wantCall: "create"},
		"delete requires auth":             {method: "DELETE", path: "/ads/1", wantStatus: http.StatusUnauthorized},
		"delete":                           {method: "DELETE", path: "/ads/1", auth: "user-1", wantStatus: http.StatusNoContent, wantCall: "delete"},
		"import requires auth":             {method: "POST", path: "/ads/import", body: `{"ads":[]}`, wantStatus: http.StatusUnauthorized},
		"import":                           {method: "POST", path: "/ads/import", body: `{"ads":[]}`, auth: "user-1", wantStatus: http.StatusCreated, wantCall: "import"},
		"import is not mistaken for an ID": {method: "GET", path: "/ads/import", wantStatus: http.StatusBadRequest},
		"updating an ad is not supported":  {method: "PUT", path: "/ads/1", auth: "user-1", wantStatus: http.StatusMethodNotAllowed},
	} {
		t.Run(name, func(t *testing.T) {
			clear(calls)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantCall == "" {
				assert.Empty(t, calls)
			} else {
				assert.Equal(t, map[string]int{tc.wantCall: 1}, calls)
			}
		})
	}
}
