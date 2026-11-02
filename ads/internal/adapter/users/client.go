package usersadapter

import (
	"context"
	"errors"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	users "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

// UsersAPIClient describes the HTTP client method required by the adapter.
type UsersAPIClient interface {
	GetUserByID(ctx context.Context, userID string) (usersapi.User, error)
}

// Client adapts the Users API client to the Ads application's user model.
type Client struct {
	client UsersAPIClient
}

// New creates a Users API adapter.
func New(client UsersAPIClient) *Client {
	return &Client{client: client}
}

// GetUserByID retrieves a user from the Users API and converts it to the Ads user model.
func (c *Client) GetUserByID(ctx context.Context, userID model.UserID) (users.User, error) {
	user, err := c.client.GetUserByID(ctx, userID.String())
	if err != nil {
		if errors.Is(err, usersapi.ErrNotFound) {
			return users.User{}, fmt.Errorf(
				"user %s not found: %w",
				userID,
				domain.ErrUserNotFound,
			)
		}

		return users.User{}, fmt.Errorf("unable to get user %s: %w", userID, err)
	}

	return users.User{
		UserID:   user.UserID,
		Nickname: user.Nickname,
	}, nil
}
