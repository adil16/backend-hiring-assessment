package usecase

import (
	"context"
	"errors"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	users "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
)

// UserGetter defines the user information required by this usecase.
type UserGetter interface {
	GetUserByID(ctx context.Context, userID model.UserID) (users.User, error)
}

type (
	// GetAdByIDFunc returns an ad enriched with its user's nickname when available.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (domain.AdWithUsername, error)

	getAdByID struct {
		adDAO      adsdao.DAO
		userClient UserGetter
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc backed by the given dependencies.
func NewGetAdByIDFunc(adDAO adsdao.DAO, userClient UserGetter) GetAdByIDFunc {
	return getAdByID{
		adDAO:      adDAO,
		userClient: userClient,
	}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (domain.AdWithUsername, error) {
	ad, err := uc.adDAO.GetAdByID(ctx, id)
	if err != nil {
		return domain.AdWithUsername{}, fmt.Errorf("get ad by ID: %w", err)
	}

	user, err := uc.userClient.GetUserByID(ctx, ad.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.AdWithUsername{Ad: ad}, nil
		}

		return domain.AdWithUsername{},
			fmt.Errorf("get user by ID: %w", err)
	}

	return domain.AdWithUsername{
		Ad:       ad,
		Username: user.Nickname,
	}, nil
}
