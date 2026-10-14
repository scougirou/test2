package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(fn)))
	return httptest.NewServer(mux)
}

func failingImport(t *testing.T) usecase.ImportAdsFunc {
	return func(context.Context, usecase.ImportAdsRequest) ([]domain.AdID, error) {
		t.Fatal("usecase should not be called")
		return nil, nil
	}
}

func TestHandlerImportAds(t *testing.T) {
	threeAds := importAdsBody{Ads: []adPayload{
		{Title: "bike", Price: ptr(int64(5000)), PhotoURL: "https://example.com/bike.jpg"},
		{Title: "scooter", Price: ptr(int64(8000)), PhotoURL: "https://example.com/scooter.jpg"},
		{Title: "car", Price: ptr(int64(200000)), PhotoURL: "https://example.com/car.jpg"},
	}}

	t.Run("returns 201 with all generated IDs on success", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return []domain.AdID{10, 11, 12}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", threeAds, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{10, 11, 12}, got.IDs)
	})

	t.Run("hands every ad to the usecase in a single call, owned by the authenticated user", func(t *testing.T) {
		var received []usecase.ImportAdsRequest
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = append(received, req)
			return []domain.AdID{1, 2, 3}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", threeAds, "user-42")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received, 1)
		assert.Equal(t, domain.UserID("user-42"), received[0].OwnerID)
		assert.Equal(t, []usecase.ImportedAd{
			{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: 8000, PhotoURL: "https://example.com/scooter.jpg"},
			{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
		}, received[0].Ads)
	})

	t.Run("accepts the former price_cents field", func(t *testing.T) {
		var received usecase.ImportAdsRequest
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{1, 2}, nil
		})
		defer srv.Close()

		body := `{"ads":[{"title":"bike","price_cents":5000,"photo_url":"u"},{"title":"scooter","price":8000,"photo_url":"u"}]}`
		resp := postRaw(t, srv.URL+"/ads/import", []byte(body), "user-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Len(t, received.Ads, 2)
		assert.Equal(t, int64(5000), received.Ads[0].PriceCents)
		assert.Equal(t, int64(8000), received.Ads[1].PriceCents)
	})

	t.Run("returns 400 naming the ad whose price is missing or ambiguous", func(t *testing.T) {
		srv := newImportAdsServer(failingImport(t))
		defer srv.Close()

		body := `{"ads":[{"title":"bike","price":1,"photo_url":"u"},{"title":"scooter","photo_url":"u"}]}`
		resp := postRaw(t, srv.URL+"/ads/import", []byte(body), "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Contains(t, decodeError(t, resp), "ads[1]")
	})

	t.Run("returns 201 with empty IDs when body has no ads", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			assert.Empty(t, req.Ads)
			return []domain.AdID{}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{Ads: []adPayload{}}, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"ids":[]}`, string(body))
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newImportAdsServer(failingImport(t))
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{}, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		srv := newImportAdsServer(failingImport(t))
		defer srv.Close()

		resp := postRaw(t, srv.URL+"/ads/import", nil, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 413 when body is too large", func(t *testing.T) {
		srv := newImportAdsServer(failingImport(t))
		defer srv.Close()

		huge := fmt.Sprintf(`{"ads":[{"title":"%s","price":1,"photo_url":"u"}]}`, strings.Repeat("a", maxImportBodyBytes))
		resp := postRaw(t, srv.URL+"/ads/import", []byte(huge), "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("returns 400 with the reason when an ad is rejected", func(t *testing.T) {
		srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, fmt.Errorf("ads[1]: %w: title must not be empty", usecase.ErrInvalidAd)
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", threeAds, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Contains(t, decodeError(t, resp), "ads[1]")
	})

	t.Run("returns 413 when the import holds too many ads", func(t *testing.T) {
		srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, fmt.Errorf("%w: an import holds at most 1000 ads", usecase.ErrTooManyAds)
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", threeAds, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newImportAdsServer(func(context.Context, usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, errors.New("db down")
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", threeAds, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
