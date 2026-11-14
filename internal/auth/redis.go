package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTimeout bounds every call to Redis. A session lookup runs on most
// requests, so a hanging Redis must fail them quickly rather than hold them.
const redisTimeout = time.Second

// startupPingTimeout gives Redis longer at start than a request may wait.
const startupPingTimeout = 5 * time.Second

// ErrStoreUnavailable marks a session or state store that cannot be reached,
// as opposed to a session or state that does not exist.
var ErrStoreUnavailable = errors.New("auth: store unavailable")

// redisClient is the part of *redis.Client the stores use, so tests can
// replace it with a client that hangs.
type redisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Close() error
}

// redisHealth logs when a store loses and regains Redis, once per transition,
// so an outage does not log a line for every request that touches the store.
type redisHealth struct {
	store string
	down  atomic.Bool
}

// observe wraps err in ErrStoreUnavailable unless Redis answered, redis.Nil
// included.
func (h *redisHealth) observe(err error) error {
	if err == nil || errors.Is(err, redis.Nil) {
		if h.down.CompareAndSwap(true, false) {
			slog.Info("redis reachable again", "store", h.store)
		}
		return err
	}
	if h.down.CompareAndSwap(false, true) {
		slog.Warn("redis unreachable", "store", h.store, "err", err)
	}
	return fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
}

// newRedisClient refuses to start without Redis: sessions and the OAuth state
// live there, so a web binary without it could not sign anybody in. An outage
// after the start only degrades the routes that need a session.
func newRedisClient(url string, health *redisHealth) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("auth: parse redis url: %w", err)
	}
	// Without this go-redis applies its own read and write timeouts and
	// ignores the deadline of redisTimeout.
	opt.ContextTimeoutEnabled = true
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), startupPingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("auth: redis ping: %w", err)
	}
	return client, nil
}
