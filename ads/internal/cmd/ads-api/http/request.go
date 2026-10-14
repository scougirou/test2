package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const (
	maxBodyBytes       = 1 << 20
	maxImportBodyBytes = 8 << 20
)

func adIDFromURL(r *http.Request) (domain.AdID, error) {
	raw := r.PathValue("adID")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ad id %q is not a number", raw)
	}

	return domain.AdID(id), nil
}

// errBodyTooLarge is returned by decodeBody when the body exceeds the allowed size.
var errBodyTooLarge = errors.New("request body too large")

func decodeBody(rw http.ResponseWriter, r *http.Request, limit int64, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(rw, r.Body, limit))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errBodyTooLarge
		}

		return fmt.Errorf("unable to decode request body: %w", err)
	}

	return nil
}

// writeBodyError answers a request whose body could not be decoded.
func writeBodyError(rw http.ResponseWriter, err error) {
	if errors.Is(err, errBodyTooLarge) {
		writeError(rw, http.StatusRequestEntityTooLarge, err.Error())

		return
	}

	writeError(rw, http.StatusBadRequest, "the request body is not valid")
}

func writeError(rw http.ResponseWriter, status int, message string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(errorResponse{Error: message})
}

type errorResponse struct {
	Error string `json:"error"`
}
