package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func ptr[T any](v T) *T { return &v }

func newCreateAdServer(fn usecase.CreateAdFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads", middleware.RequireAuth(handlerCreateAd(fn)))
	return httptest.NewServer(mux)
}

func postJSON(t *testing.T, url string, body any, authHeader string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	return postRaw(t, url, b, authHeader)
}

func postRaw(t *testing.T, url string, body []byte, authHeader string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeError(t *testing.T, resp *http.Response) string {
	t.Helper()

	var got errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	return got.Error
}

func TestHandlerCreateAd(t *testing.T) {
	validBody := adPayload{
		Title:    "mountain bike",
		Price:    ptr(int64(15000)),
		PhotoURL: "https://example.com/bike.jpg",
	}

	t.Run("returns 201 with ID on success", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			return domain.AdID(7), nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var got createAdResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, int64(7), got.ID)
	})

	t.Run("passes authenticated user ID as owner to the usecase", func(t *testing.T) {
		var received usecase.CreateAdRequest
		srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
			received = req
			return domain.AdID(1), nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-42")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, domain.UserID("user-42"), received.OwnerID)
		assert.Equal(t, "mountain bike", received.Title)
		assert.Equal(t, int64(15000), received.PriceCents)
		assert.Equal(t, "https://example.com/bike.jpg", received.PhotoURL)
	})

	t.Run("price", func(t *testing.T) {
		for name, tc := range map[string]struct {
			body      string
			wantPrice int64
			wantCode  int
		}{
			"accepts price":                      {body: `{"title":"bike","price":15000,"photo_url":"u"}`, wantPrice: 15000, wantCode: http.StatusCreated},
			"accepts the former price_cents":     {body: `{"title":"bike","price_cents":15000,"photo_url":"u"}`, wantPrice: 15000, wantCode: http.StatusCreated},
			"accepts both when they agree":       {body: `{"title":"bike","price":15000,"price_cents":15000,"photo_url":"u"}`, wantPrice: 15000, wantCode: http.StatusCreated},
			"accepts a price of zero":            {body: `{"title":"bike","price":0,"photo_url":"u"}`, wantPrice: 0, wantCode: http.StatusCreated},
			"rejects both when they disagree":    {body: `{"title":"bike","price":1,"price_cents":2,"photo_url":"u"}`, wantCode: http.StatusBadRequest},
			"rejects a missing price":            {body: `{"title":"bike","photo_url":"u"}`, wantCode: http.StatusBadRequest},
			"rejects a price that is not number": {body: `{"title":"bike","price":"free","photo_url":"u"}`, wantCode: http.StatusBadRequest},
		} {
			t.Run(name, func(t *testing.T) {
				var received usecase.CreateAdRequest
				called := false
				srv := newCreateAdServer(func(_ context.Context, req usecase.CreateAdRequest) (domain.AdID, error) {
					called, received = true, req
					return domain.AdID(1), nil
				})
				defer srv.Close()

				resp := postRaw(t, srv.URL+"/ads", []byte(tc.body), "user-1")
				defer resp.Body.Close()

				assert.Equal(t, tc.wantCode, resp.StatusCode)
				assert.Equal(t, tc.wantCode == http.StatusCreated, called)
				if called {
					assert.Equal(t, tc.wantPrice, received.PriceCents)
				}
			})
		}
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 when body is invalid JSON", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		resp := postRaw(t, srv.URL+"/ads", []byte("not json"), "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when body has an unknown field", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		resp := postRaw(t, srv.URL+"/ads", []byte(`{"title":"bike","price":1,"photo_url":"u","user_id":"someone-else"}`), "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 413 when body is too large", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return 0, nil
		})
		defer srv.Close()

		huge := fmt.Sprintf(`{"title":"%s","price":1,"photo_url":"u"}`, strings.Repeat("a", maxBodyBytes))
		resp := postRaw(t, srv.URL+"/ads", []byte(huge), "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("returns 400 with the reason when the ad is rejected", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, fmt.Errorf("%w: title must not be empty", usecase.ErrInvalidAd)
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Contains(t, decodeError(t, resp), "title must not be empty")
	})

	t.Run("returns 429 with Retry-After when the quota is reached", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, fmt.Errorf("unable to create ad: %w", &usecase.QuotaExceededError{RetryAfter: 90*time.Minute + 200*time.Millisecond})
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		assert.Equal(t, "5401", resp.Header.Get("Retry-After"))
		assert.NotEmpty(t, decodeError(t, resp))
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newCreateAdServer(func(_ context.Context, _ usecase.CreateAdRequest) (domain.AdID, error) {
			return 0, errors.New("db down")
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads", validBody, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
