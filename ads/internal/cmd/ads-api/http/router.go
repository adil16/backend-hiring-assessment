package http

import (
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

// Usecases groups the business functions the HTTP layer depends on.
type Usecases struct {
	ListAds  usecase.ListAdsFunc
	GetAd    usecase.GetAdByIDFunc
	CreateAd usecase.CreateAdFunc
}

// NewRouter wires the routes of the API.
func NewRouter(usecases Usecases) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /ads", handlerListAds(usecases.ListAds))
	mux.Handle("GET /ads/{adID}", handlerGetAd(usecases.GetAd))
	mux.Handle("POST /ads", middleware.RequireAuth(handlerCreateAd(usecases.CreateAd)))
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(usecases.CreateAd)))

	mux.HandleFunc("GET /health", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	return mux
}
