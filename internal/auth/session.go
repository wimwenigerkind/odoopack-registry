package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

const SessionCookieName = "session"

var ErrSessionNotFound = errors.New("session not found or expired")

type Session struct {
	ID        string
	UserID    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
	UserAgent string
	IP        string
}

type SessionStore struct {
	mu       sync.Mutex
	items    map[string]Session
	stop     chan struct{}
	stopOnce sync.Once
}

func NewSessionStore() *SessionStore {
	s := &SessionStore{
		items: make(map[string]Session),
		stop:  make(chan struct{}),
	}
	go s.janitor()
	return s
}

func (s *SessionStore) Create(userID uuid.UUID, ttl time.Duration, userAgent, ip string) (Session, error) {
	id := NewState()
	now := time.Now()
	sess := Session{
		ID:        id,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UserAgent: userAgent,
		IP:        ip,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = sess
	return sess, nil
}

func (s *SessionStore) Get(id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.items[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if time.Now().After(sess.ExpiresAt) {
		delete(s.items, id)
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *SessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

func (s *SessionStore) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

func (s *SessionStore) janitor() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			now := time.Now()
			s.mu.Lock()
			for k, sess := range s.items {
				if now.After(sess.ExpiresAt) {
					delete(s.items, k)
				}
			}
			s.mu.Unlock()
		}
	}
}
