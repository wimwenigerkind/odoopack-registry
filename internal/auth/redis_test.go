package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// hangingRedis answers no call until the caller's context ends, like a Redis
// that accepts connections but never replies.
type hangingRedis struct{}

func (hangingRedis) Get(ctx context.Context, _ string) *redis.StringCmd {
	<-ctx.Done()
	return redis.NewStringResult("", ctx.Err())
}

func (hangingRedis) GetDel(ctx context.Context, _ string) *redis.StringCmd {
	<-ctx.Done()
	return redis.NewStringResult("", ctx.Err())
}

func (hangingRedis) Set(ctx context.Context, _ string, _ any, _ time.Duration) *redis.StatusCmd {
	<-ctx.Done()
	return redis.NewStatusResult("", ctx.Err())
}

func (hangingRedis) Del(ctx context.Context, _ ...string) *redis.IntCmd {
	<-ctx.Done()
	return redis.NewIntResult(0, ctx.Err())
}

func (hangingRedis) Close() error { return nil }

func TestRedisStoresGiveUpAfterTimeout(t *testing.T) {
	sessions := &redisSessionStore{client: hangingRedis{}, health: &redisHealth{store: "session"}}
	states := &redisStateStore{client: hangingRedis{}, health: &redisHealth{store: "oauth state"}}

	calls := map[string]func() error{
		"session get": func() error { _, err := sessions.Get("abc"); return err },
		"session create": func() error {
			_, err := sessions.Create(uuid.New(), time.Hour, "", "")
			return err
		},
		"session delete": func() error { return sessions.Delete("abc") },
		"state save":     func() error { return states.Save("abc", FlowState{}, time.Minute) },
		"state take":     func() error { _, err := states.Take("abc"); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			err := call()
			if elapsed := time.Since(start); elapsed > redisTimeout+500*time.Millisecond {
				t.Errorf("returned after %s, want about %s", elapsed, redisTimeout)
			}
			if !errors.Is(err, ErrStoreUnavailable) {
				t.Errorf("err = %v, want ErrStoreUnavailable", err)
			}
		})
	}
}

func TestRedisHealthKeepsNotFoundApart(t *testing.T) {
	h := &redisHealth{store: "session"}
	if err := h.observe(redis.Nil); !errors.Is(err, redis.Nil) || errors.Is(err, ErrStoreUnavailable) {
		t.Errorf("observe(redis.Nil) = %v, want redis.Nil only", err)
	}
	if err := h.observe(errors.New("connection refused")); !errors.Is(err, ErrStoreUnavailable) {
		t.Errorf("observe(connection refused) = %v, want ErrStoreUnavailable", err)
	}
	if !h.down.Load() {
		t.Error("store not marked down after a failed call")
	}
	if err := h.observe(nil); err != nil || h.down.Load() {
		t.Errorf("observe(nil) = %v, down = %v, want nil and up again", err, h.down.Load())
	}
}

func TestStoresRefuseToStartWithoutRedis(t *testing.T) {
	// Port 1 refuses connections, so the startup ping fails right away.
	const unreachable = "redis://127.0.0.1:1/0"
	if _, err := NewRedisSessionStore(unreachable); err == nil {
		t.Error("session store started without Redis")
	}
	if _, err := NewRedisStateStore(unreachable); err == nil {
		t.Error("state store started without Redis")
	}
}
