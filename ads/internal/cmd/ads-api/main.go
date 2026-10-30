package main

import (
	"context"
	"log/slog"
	"os"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/http"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/httpserver"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg := Load()

	adDAO, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := adDAO.Close(); err != nil {
			slog.Error("unable to close the database", "error", err)
		}
	}()

	router := http.NewRouter(http.Usecases{
		CreateAd: usecase.NewCreateAdFunc(adDAO),
		GetAd:    usecase.NewGetAdByIDFunc(adDAO),
		ListAds:  usecase.NewListAdsFunc(adDAO),
	})

	return httpserver.Run(ctx, cfg.HTTPAddr, router)
}
