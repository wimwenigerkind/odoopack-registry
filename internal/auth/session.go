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
	ID        string    `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
}

type SessionStore interface {
	Create(userID uuid.UUID, ttl time.Duration, userAgent, ip string) (Session, error)
	Get(id string) (Session, error)
	Delete(id string) error
	Stop()
}

type memorySessionStore struct {
	mu       sync.Mutex
	items    map[string]Session
	stop     chan struct{}
	stopOnce sync.Once
}

func NewMemorySessionStore() SessionStore {
	s := &memorySessionStore{
		items: make(map[string]Session),
		stop:  make(chan struct{}),
	}
	go s.janitor()
	return s
}

func (s *memorySessionStore) Create(userID uuid.UUID, ttl time.Duration, userAgent, ip string) (Session, error) {
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

func (s *memorySessionStore) Get(id string) (Session, error) {
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

func (s *memorySessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

func (s *memorySessionStore) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

func (s *memorySessionStore) janitor() {
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
