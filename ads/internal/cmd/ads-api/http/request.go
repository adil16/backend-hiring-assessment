package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

func adIDFromURL(r *http.Request) (domain.AdID, error) {
	raw := r.PathValue("adID")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ad id %q is not a number", raw)
	}

	return domain.AdID(id), nil
}

func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("unable to decode request body: %w", err)
	}

	return nil
}
