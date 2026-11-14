package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

type redisSessionStore struct {
	client redisClient
	health *redisHealth
}

func NewRedisSessionStore(url string) (SessionStore, error) {
	health := &redisHealth{store: "session"}
	client, err := newRedisClient(url, health)
	if err != nil {
		return nil, err
	}
	return &redisSessionStore{client: client, health: health}, nil
}

func (s *redisSessionStore) key(id string) string {
	return sessionKeyPrefix + id
}

func (s *redisSessionStore) Create(userID uuid.UUID, ttl time.Duration, userAgent, ip string) (Session, error) {
	now := time.Now()
	sess := Session{
		ID:        NewState(),
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UserAgent: userAgent,
		IP:        ip,
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return Session{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	if err := s.health.observe(s.client.Set(ctx, s.key(sess.ID), data, ttl).Err()); err != nil {
		return Session{}, fmt.Errorf("auth: redis set session: %w", err)
	}
	return sess, nil
}

func (s *redisSessionStore) Get(id string) (Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	data, err := s.client.Get(ctx, s.key(id)).Bytes()
	err = s.health.observe(err)
	if errors.Is(err, redis.Nil) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("auth: redis get session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return Session{}, err
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.client.Del(ctx, s.key(id)).Err()
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *redisSessionStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	return s.health.observe(s.client.Del(ctx, s.key(id)).Err())
}

func (s *redisSessionStore) Stop() {
	_ = s.client.Close()
}
