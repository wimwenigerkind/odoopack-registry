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
	client redisClient
	health *redisHealth
}

func NewRedisStateStore(url string) (StateStore, error) {
	health := &redisHealth{store: "oauth state"}
	client, err := newRedisClient(url, health)
	if err != nil {
		return nil, err
	}
	return &redisStateStore{client: client, health: health}, nil
}

func (s *redisStateStore) key(state string) string {
	return stateKeyPrefix + state
}

func (s *redisStateStore) Save(state string, fs FlowState, ttl time.Duration) error {
	data, err := json.Marshal(fs)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	if err := s.health.observe(s.client.Set(ctx, s.key(state), data, ttl).Err()); err != nil {
		return fmt.Errorf("auth: redis set state: %w", err)
	}
	return nil
}

func (s *redisStateStore) Take(state string) (FlowState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	data, err := s.client.GetDel(ctx, s.key(state)).Bytes()
	err = s.health.observe(err)
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
