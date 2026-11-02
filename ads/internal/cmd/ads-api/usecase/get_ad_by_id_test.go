package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersmock "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/mock"
)

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		daoMock := mockdao.New(t)
		userMock := usersmock.New(t)

		ad := domain.Ad{
			ID:     42,
			Title:  "bike",
			Price:  5000,
			UserID: "user-1",
		}

		user := usersclient.User{
			UserID:   "user-1",
			Nickname: "alice",
		}

		want := domain.AdWithUsername{
			Ad:       ad,
			Username: user.Nickname,
		}

		daoMock.On("GetAdByID", ctx, ad.ID).Return(ad, nil).Once()
		userMock.On("GetUserByID", ctx, ad.UserID).Return(user, nil).Once()

		got, err := usecase.NewGetAdByIDFunc(daoMock, userMock)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns error when ad is not found", func(t *testing.T) {
		daoMock := mockdao.New(t)
		userMock := usersmock.New(t)

		daoMock.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, adsdao.ErrNotFound).Once()

		_, err := usecase.NewGetAdByIDFunc(daoMock, userMock)(ctx, 99)

		require.ErrorIs(t, err, adsdao.ErrNotFound)
		userMock.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
	})

	t.Run("returns error when dao fails", func(t *testing.T) {
		daoMock := mockdao.New(t)
		userMock := usersmock.New(t)

		dbErr := errors.New("connection reset")

		daoMock.On("GetAdByID", ctx, domain.AdID(1)).Return(domain.Ad{}, dbErr).Once()

		_, err := usecase.NewGetAdByIDFunc(daoMock, userMock)(ctx, 1)

		require.ErrorIs(t, err, dbErr)
		userMock.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
	})

	t.Run("returns ad without username when user is not found", func(t *testing.T) {
		daoMock := mockdao.New(t)
		userMock := usersmock.New(t)

		ad := domain.Ad{
			ID:     42,
			Title:  "bike",
			Price:  5000,
			UserID: "user-1",
		}

		daoMock.On("GetAdByID", ctx, ad.ID).Return(ad, nil).Once()
		userMock.On("GetUserByID", ctx, ad.UserID).Return(usersclient.User{}, domain.ErrUserNotFound).Once()

		got, err := usecase.NewGetAdByIDFunc(daoMock, userMock)(ctx, ad.ID)

		require.NoError(t, err)
		assert.Equal(t, domain.AdWithUsername{Ad: ad}, got)
	})

	t.Run("returns error when user service fails", func(t *testing.T) {
		daoMock := mockdao.New(t)
		userMock := usersmock.New(t)

		ad := domain.Ad{
			ID:     42,
			Title:  "bike",
			Price:  5000,
			UserID: "user-1",
		}

		wantErr := errors.New("connection reset")

		daoMock.On("GetAdByID", ctx, ad.ID).Return(ad, nil).Once()
		userMock.On("GetUserByID", ctx, ad.UserID).Return(usersclient.User{}, wantErr).Once()

		got, err := usecase.NewGetAdByIDFunc(daoMock, userMock)(ctx, ad.ID)

		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, domain.AdWithUsername{}, got)
	})
}
