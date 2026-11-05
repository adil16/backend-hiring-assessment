package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newDeleteAdServer(fn usecase.DeleteAdByIDFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("DELETE /ads/{adID}", middleware.RequireAuth(middleware.RequireValidUUIDUserID(handlerDeleteAd(fn))))
	return httptest.NewServer(mux)
}

func TestHandlerDeleteAd(t *testing.T) {

	t.Run("returns 200 on success", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, id domain.AdID, userID domain.UserID) error {
			assert.Equal(t, domain.AdID(42), id)
			assert.Equal(t, domain.UserID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), userID)
			return nil
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/42",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, id domain.AdID, userID domain.UserID) error {
			return nil
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/42",
			nil,
		)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 when Authorization header contains an invalid UUID", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ domain.AdID, _ domain.UserID) error {
			t.Error("usecase should not be called")
			return nil
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/42",
			nil,
		)
		require.NoError(t, err)

		req.Header.Set("Authorization", "invaliduser")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when adID is not a number", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ domain.AdID, userID domain.UserID) error {
			t.Fatal("usecase should not be called")
			return nil
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/not-a-number",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "00000000-0000-0000-0000-000000000123")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 404 when ad does not exist", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ domain.AdID, userID domain.UserID) error {
			return adsdao.ErrNotFound
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/99",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "00000000-0000-0000-0000-000000000123")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns 403 when ad does not belong to user", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ domain.AdID, userID domain.UserID) error {
			return fmt.Errorf("unable to delete ad 99: user 00000000-0000-0000-0000-000000000123 is not the owner: %w", domain.ErrForbidden)
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/99",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "00000000-0000-0000-0000-000000000123")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("returns 500 on unexpected usecase error", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ domain.AdID, userID domain.UserID) error {
			return errors.New("db down")
		})
		t.Cleanup(srv.Close)

		req, err := http.NewRequest(
			http.MethodDelete,
			srv.URL+"/ads/1",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "00000000-0000-0000-0000-000000000123")

		resp, err := http.DefaultClient.Do(req)
		requireHTTPResponse(t, resp, err)

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
