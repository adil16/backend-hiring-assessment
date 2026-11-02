package main

import (
	"context"
	"log/slog"
	"os"

	usersadapter "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/adapter/users"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/http"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/httpserver"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/version"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

func main() {
	slog.Info("starting ads-api",
		"version", version.Version,
		"commit", version.GitCommit,
		"build_date", version.BuildDate,
	)

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

	userAPIClient := usersapi.NewClient(cfg.IdentityURL, cfg.IdentityTimeout)
	userClient := usersadapter.New(userAPIClient)

	router := http.NewRouter(http.Usecases{
		CreateAd: usecase.NewCreateAdFunc(adDAO),
		GetAd:    usecase.NewGetAdByIDFunc(adDAO, userClient),
		ListAds:  usecase.NewListAdsFunc(adDAO),
		DeleteAd: usecase.NewDeleteAdByIDFunc(adDAO),
	})

	return httpserver.Run(ctx, cfg.HTTPAddr, router)
}
