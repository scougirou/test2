// Package identity connects the ads service to the user service.
package identity

import (
	"context"
	"net/url"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

// userService is the subset of the HTTP client of the user service the adapter needs.
type userService interface {
	GetUserByID(ctx context.Context, userID string) (api.User, error)
	ListUsers(ctx context.Context) ([]api.User, error)
}

// Client adapts the HTTP client of the user service to the usersclient.UserClient interface. It reports an unknown user with api.ErrNotFound.
type Client struct {
	service userService
}

var _ usersclient.UserClient = (*Client)(nil)

// NewClient returns a client calling the user service at baseURL. Requests exceeding timeout are aborted.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{service: api.NewClient(baseURL, timeout)}
}

// GetUserByID returns the user with the given ID.
func (c *Client) GetUserByID(ctx context.Context, userID model.UserID) (usersclient.User, error) {
	user, err := c.service.GetUserByID(ctx, url.PathEscape(userID.String()))
	if err != nil {
		return usersclient.User{}, err
	}

	return usersclient.User{UserID: user.UserID, Nickname: user.Nickname}, nil
}

// ListUsers returns all the users.
func (c *Client) ListUsers(ctx context.Context) ([]usersclient.User, error) {
	users, err := c.service.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	converted := make([]usersclient.User, 0, len(users))
	for _, user := range users {
		converted = append(converted, usersclient.User{UserID: user.UserID, Nickname: user.Nickname})
	}

	return converted, nil
}
