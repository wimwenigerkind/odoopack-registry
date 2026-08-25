package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const stateKeyPrefix = "oauth_state:"

type redisStateStore struct {
	client *redis.Client
}

func NewRedisStateStore(url string) (StateStore, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("auth: parse redis url: %w", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("auth: redis ping: %w", err)
	}
	return &redisStateStore{client: client}, nil
}

func (s *redisStateStore) key(state string) string {
	return stateKeyPrefix + state
}

func (s *redisStateStore) Save(state string, fs FlowState, ttl time.Duration) error {
	data, err := json.Marshal(fs)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.client.Set(ctx, s.key(state), data, ttl).Err(); err != nil {
		return fmt.Errorf("auth: redis set state: %w", err)
	}
	return nil
}

func (s *redisStateStore) Take(state string) (FlowState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := s.client.GetDel(ctx, s.key(state)).Bytes()
	if errors.Is(err, redis.Nil) {
		return FlowState{}, ErrStateNotFound
	}
	if err != nil {
		return FlowState{}, fmt.Errorf("auth: redis getdel state: %w", err)
	}
	var fs FlowState
	if err := json.Unmarshal(data, &fs); err != nil {
		return FlowState{}, err
	}
	return fs, nil
}

func (s *redisStateStore) Stop() {
	_ = s.client.Close()
}
