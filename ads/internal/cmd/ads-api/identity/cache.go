package identity

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

const (
	// userTTL is how long a user is remembered. Nicknames rarely change, so a stale one is harmless.
	userTTL = 5 * time.Minute
	// notFoundTTL is how long an unknown user is remembered.
	notFoundTTL = time.Minute
	// errorTTL is how long a failure is remembered. It is short, but it keeps a struggling user service from being hit by every read.
	errorTTL = 5 * time.Second
	// maxEntries bounds the memory used by the cache.
	maxEntries = 10_000
)

type entry struct {
	user      usersclient.User
	err       error
	expiresAt time.Time
}

// CachedClient remembers the answers of another client for a while and sends a single call for concurrent lookups of the same user, so that reading ads does not turn into as many calls to the user service.
type CachedClient struct {
	next usersclient.UserClient
	now  func() time.Time

	mu      sync.Mutex
	entries map[model.UserID]entry
	calls   singleflight.Group
}

var _ usersclient.UserClient = (*CachedClient)(nil)

// NewCachedClient wraps next with a cache.
func NewCachedClient(next usersclient.UserClient) *CachedClient {
	return &CachedClient{next: next, now: time.Now, entries: make(map[model.UserID]entry)}
}

// GetUserByID returns the user with the given ID, from the cache when possible.
func (c *CachedClient) GetUserByID(ctx context.Context, userID model.UserID) (usersclient.User, error) {
	if cached, ok := c.lookup(userID); ok {
		return cached.user, cached.err
	}

	// The call is shared between callers, so it must not die with the context of the first one. The next client is responsible for its own timeout.
	result := c.calls.DoChan(userID.String(), func() (any, error) {
		user, err := c.next.GetUserByID(context.WithoutCancel(ctx), userID)
		c.store(userID, user, err)

		return user, err
	})

	select {
	case <-ctx.Done():
		return usersclient.User{}, ctx.Err()
	case res := <-result:
		user, _ := res.Val.(usersclient.User)

		return user, res.Err
	}
}

// ListUsers returns all the users. It is not cached.
func (c *CachedClient) ListUsers(ctx context.Context) ([]usersclient.User, error) {
	return c.next.ListUsers(ctx)
}

func (c *CachedClient) lookup(userID model.UserID) (entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached, ok := c.entries[userID]
	if !ok || !c.now().Before(cached.expiresAt) {
		return entry{}, false
	}

	return cached, true
}

func (c *CachedClient) store(userID model.UserID, user usersclient.User, err error) {
	ttl := userTTL

	switch {
	case errors.Is(err, api.ErrNotFound):
		ttl = notFoundTTL
	case err != nil:
		ttl = errorTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()

	if len(c.entries) >= maxEntries {
		for id, cached := range c.entries {
			if !now.Before(cached.expiresAt) {
				delete(c.entries, id)
			}
		}
	}

	if len(c.entries) >= maxEntries {
		clear(c.entries)
	}

	c.entries[userID] = entry{user: user, err: err, expiresAt: now.Add(ttl)}
}
