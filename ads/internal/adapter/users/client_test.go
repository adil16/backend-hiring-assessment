package usersadapter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

type fakeAPIClient struct {
	user       usersapi.User
	err        error
	receivedID string
}

func (f *fakeAPIClient) GetUserByID(_ context.Context, userID string) (usersapi.User, error) {
	f.receivedID = userID
	return f.user, f.err
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns user on success", func(t *testing.T) {
		fake := &fakeAPIClient{
			user: usersapi.User{
				UserID:   model.UserID("user-1"),
				Nickname: "alice",
			},
		}

		client := New(fake)

		got, err := client.GetUserByID(ctx, model.UserID("user-1"))

		require.NoError(t, err)
		assert.Equal(t, usersclient.User{
			UserID:   model.UserID("user-1"),
			Nickname: "alice",
		}, got)
		assert.Equal(t, "user-1", fake.receivedID)
	})

	t.Run("translates not found error", func(t *testing.T) {
		fake := &fakeAPIClient{
			err: usersapi.ErrNotFound,
		}

		client := New(fake)

		got, err := client.GetUserByID(ctx, model.UserID("unknown"))

		require.ErrorIs(t, err, domain.ErrUserNotFound)
		assert.Equal(t, usersclient.User{}, got)
		assert.Equal(t, "unknown", fake.receivedID)
	})

	t.Run("translates unexpected error", func(t *testing.T) {
		wantErr := errors.New("connection reset")

		fake := &fakeAPIClient{
			err: wantErr,
		}

		client := New(fake)

		got, err := client.GetUserByID(ctx, model.UserID("user-1"))

		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, usersclient.User{}, got)
	})
}
