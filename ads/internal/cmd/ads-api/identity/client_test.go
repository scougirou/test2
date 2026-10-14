package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

type fakeService struct {
	getUser   func(ctx context.Context, userID string) (api.User, error)
	listUsers func(ctx context.Context) ([]api.User, error)
}

func (f fakeService) GetUserByID(ctx context.Context, userID string) (api.User, error) {
	return f.getUser(ctx, userID)
}

func (f fakeService) ListUsers(ctx context.Context) ([]api.User, error) {
	return f.listUsers(ctx)
}

func TestClientGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("converts the user", func(t *testing.T) {
		client := &Client{service: fakeService{getUser: func(_ context.Context, userID string) (api.User, error) {
			assert.Equal(t, "user-1", userID)
			return api.User{UserID: "user-1", Nickname: "alice"}, nil
		}}}

		got, err := client.GetUserByID(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, usersclient.User{UserID: "user-1", Nickname: "alice"}, got)
	})

	t.Run("escapes the identifier before it ends up in a URL", func(t *testing.T) {
		client := &Client{service: fakeService{getUser: func(_ context.Context, userID string) (api.User, error) {
			assert.Equal(t, "a%2Fb%3Fc", userID)
			return api.User{}, api.ErrNotFound
		}}}

		_, err := client.GetUserByID(ctx, "a/b?c")

		assert.ErrorIs(t, err, api.ErrNotFound)
	})

	t.Run("reports an unknown user with api.ErrNotFound", func(t *testing.T) {
		client := &Client{service: fakeService{getUser: func(context.Context, string) (api.User, error) {
			return api.User{}, api.ErrNotFound
		}}}

		_, err := client.GetUserByID(ctx, "ghost")

		assert.ErrorIs(t, err, api.ErrNotFound)
	})

	t.Run("returns other errors", func(t *testing.T) {
		boom := errors.New("identity service answered 503")
		client := &Client{service: fakeService{getUser: func(context.Context, string) (api.User, error) {
			return api.User{}, boom
		}}}

		_, err := client.GetUserByID(ctx, "user-1")

		assert.ErrorIs(t, err, boom)
	})
}

func TestClientListUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("converts the users", func(t *testing.T) {
		client := &Client{service: fakeService{listUsers: func(context.Context) ([]api.User, error) {
			return []api.User{{UserID: "user-1", Nickname: "alice"}, {UserID: "user-2", Nickname: "bob"}}, nil
		}}}

		got, err := client.ListUsers(ctx)

		require.NoError(t, err)
		assert.Equal(t, []usersclient.User{{UserID: "user-1", Nickname: "alice"}, {UserID: "user-2", Nickname: "bob"}}, got)
	})

	t.Run("returns errors", func(t *testing.T) {
		boom := errors.New("identity service answered 503")
		client := &Client{service: fakeService{listUsers: func(context.Context) ([]api.User, error) {
			return nil, boom
		}}}

		got, err := client.ListUsers(ctx)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})
}
