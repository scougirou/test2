package identity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
	mockusers "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/mock"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestCache(next usersclient.UserClient) (*CachedClient, *clock) {
	c := &clock{now: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	cache := NewCachedClient(next)
	cache.now = c.Now

	return cache, c
}

// The cache hands the next client a context detached from the caller's, so the expectations below accept any context.
func TestCachedClientGetUserByID(t *testing.T) {
	ctx := context.Background()
	alice := usersclient.User{UserID: "user-1", Nickname: "alice"}

	t.Run("asks the user service once while the answer is fresh", func(t *testing.T) {
		next := mockusers.New(t)
		next.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Once()
		cache, clk := newTestCache(next)

		for range 3 {
			got, err := cache.GetUserByID(ctx, "user-1")

			require.NoError(t, err)
			assert.Equal(t, alice, got)
			clk.Advance(userTTL / 4)
		}
	})

	t.Run("asks again once the answer is stale", func(t *testing.T) {
		next := mockusers.New(t)
		renamed := usersclient.User{UserID: "user-1", Nickname: "alice2"}
		next.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Once()
		next.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(renamed, nil).Once()
		cache, clk := newTestCache(next)

		first, err := cache.GetUserByID(ctx, "user-1")
		require.NoError(t, err)
		clk.Advance(userTTL)
		second, err := cache.GetUserByID(ctx, "user-1")
		require.NoError(t, err)

		assert.Equal(t, alice, first)
		assert.Equal(t, renamed, second)
	})

	t.Run("keeps users apart", func(t *testing.T) {
		next := mockusers.New(t)
		bob := usersclient.User{UserID: "user-2", Nickname: "bob"}
		next.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Once()
		next.On("GetUserByID", mock.Anything, model.UserID("user-2")).Return(bob, nil).Once()
		cache, _ := newTestCache(next)

		first, err := cache.GetUserByID(ctx, "user-1")
		require.NoError(t, err)
		second, err := cache.GetUserByID(ctx, "user-2")
		require.NoError(t, err)

		assert.Equal(t, alice, first)
		assert.Equal(t, bob, second)
	})

	t.Run("remembers an unknown user for a while", func(t *testing.T) {
		next := mockusers.New(t)
		next.On("GetUserByID", mock.Anything, model.UserID("ghost")).Return(usersclient.User{}, api.ErrNotFound).Twice()
		cache, clk := newTestCache(next)

		_, err := cache.GetUserByID(ctx, "ghost")
		assert.ErrorIs(t, err, api.ErrNotFound)
		clk.Advance(notFoundTTL - time.Second)
		_, err = cache.GetUserByID(ctx, "ghost")
		assert.ErrorIs(t, err, api.ErrNotFound)

		clk.Advance(time.Second)
		_, err = cache.GetUserByID(ctx, "ghost")
		assert.ErrorIs(t, err, api.ErrNotFound)
	})

	t.Run("remembers a failure only briefly", func(t *testing.T) {
		next := mockusers.New(t)
		boom := errors.New("identity service answered 503")
		next.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(usersclient.User{}, boom).Once()
		next.On("GetUserByID", mock.Anything, model.UserID("user-1")).Return(alice, nil).Once()
		cache, clk := newTestCache(next)

		_, err := cache.GetUserByID(ctx, "user-1")
		assert.ErrorIs(t, err, boom)
		clk.Advance(errorTTL - time.Second)
		_, err = cache.GetUserByID(ctx, "user-1")
		assert.ErrorIs(t, err, boom)

		clk.Advance(time.Second)
		got, err := cache.GetUserByID(ctx, "user-1")
		require.NoError(t, err)
		assert.Equal(t, alice, got)
	})

	t.Run("never holds more than its maximum number of entries", func(t *testing.T) {
		next := mockusers.New(t)
		next.On("GetUserByID", mock.Anything, mock.Anything).Return(alice, nil)
		cache, _ := newTestCache(next)

		for i := range maxEntries + 10 {
			_, err := cache.GetUserByID(ctx, model.UserID(fmt.Sprintf("user-%d", i)))
			require.NoError(t, err)
		}

		assert.LessOrEqual(t, len(cache.entries), maxEntries)
	})
}

type blockingClient struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	user    usersclient.User
}

func (b *blockingClient) GetUserByID(context.Context, model.UserID) (usersclient.User, error) {
	if b.calls.Add(1) == 1 {
		close(b.started)
	}
	<-b.release

	return b.user, nil
}

func (b *blockingClient) ListUsers(context.Context) ([]usersclient.User, error) { return nil, nil }

func TestCachedClientConcurrentLookups(t *testing.T) {
	alice := usersclient.User{UserID: "user-1", Nickname: "alice"}

	t.Run("sends a single call for concurrent lookups of the same user", func(t *testing.T) {
		next := &blockingClient{started: make(chan struct{}), release: make(chan struct{}), user: alice}
		cache, _ := newTestCache(next)

		const callers = 20
		var wg sync.WaitGroup
		results := make([]usersclient.User, callers)
		errs := make([]error, callers)

		for i := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = cache.GetUserByID(context.Background(), "user-1")
			}()
		}

		<-next.started
		time.Sleep(50 * time.Millisecond) // lets the other callers join the call in flight
		close(next.release)
		wg.Wait()

		for i := range callers {
			require.NoError(t, errs[i])
			assert.Equal(t, alice, results[i])
		}
		assert.EqualValues(t, 1, next.calls.Load())
	})

	t.Run("lets a caller give up without cancelling the shared call", func(t *testing.T) {
		next := &blockingClient{started: make(chan struct{}), release: make(chan struct{}), user: alice}
		cache, _ := newTestCache(next)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := cache.GetUserByID(ctx, "user-1")
			done <- err
		}()

		<-next.started
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)

		close(next.release)

		require.Eventually(t, func() bool {
			_, ok := cache.lookup("user-1")
			return ok
		}, time.Second, 5*time.Millisecond)

		got, err := cache.GetUserByID(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, alice, got)
		assert.EqualValues(t, 1, next.calls.Load())
	})
}

func TestCachedClientListUsers(t *testing.T) {
	ctx := context.Background()
	next := mockusers.New(t)
	users := []usersclient.User{{UserID: "user-1", Nickname: "alice"}}
	next.On("ListUsers", ctx).Return(users, nil).Twice()
	cache, _ := newTestCache(next)

	for range 2 {
		got, err := cache.ListUsers(ctx)

		require.NoError(t, err)
		assert.Equal(t, users, got)
	}
}
