package http

import (
	"errors"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func handlerDeleteAd(deleteAd usecase.DeleteAdByIDFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		userID := domain.UserID(middleware.UserIDFromRequest(r))

		err = deleteAd(r.Context(), adID, userID)
		if err != nil {
			if errors.Is(err, adsdao.ErrNotFound) {
				rw.WriteHeader(http.StatusNotFound)
			} else if errors.Is(err, domain.ErrForbidden) {
				rw.WriteHeader(http.StatusForbidden)
			} else {
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

	}
}
