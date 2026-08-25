package auth

import (
	"errors"
	"sync"
	"time"
)

var ErrStateNotFound = errors.New("oauth state not found or expired")

type FlowKind string

const (
	FlowLogin       FlowKind = "login"
	FlowIntegration FlowKind = "integration"
	FlowLink        FlowKind = "link"
)

type FlowState struct {
	Provider string
	Kind     FlowKind
	Nonce    string
	Verifier string
	ReturnTo string
	UserID   string
}

type StateStore interface {
	Save(state string, fs FlowState, ttl time.Duration) error
	Take(state string) (FlowState, error)
	Stop()
}

type stateEntry struct {
	fs        FlowState
	expiresAt time.Time
}

type memoryStateStore struct {
	mu       sync.Mutex
	items    map[string]stateEntry
	stop     chan struct{}
	stopOnce sync.Once
}

func NewMemoryStateStore() StateStore {
	s := &memoryStateStore{
		items: make(map[string]stateEntry),
		stop:  make(chan struct{}),
	}
	go s.janitor()
	return s
}

func (s *memoryStateStore) Save(state string, fs FlowState, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[state] = stateEntry{fs: fs, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *memoryStateStore) Take(state string) (FlowState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[state]
	if !ok {
		return FlowState{}, ErrStateNotFound
	}
	delete(s.items, state)
	if time.Now().After(e.expiresAt) {
		return FlowState{}, ErrStateNotFound
	}
	return e.fs, nil
}

func (s *memoryStateStore) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

func (s *memoryStateStore) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			now := time.Now()
			s.mu.Lock()
			for k, e := range s.items {
				if now.After(e.expiresAt) {
					delete(s.items, k)
				}
			}
			s.mu.Unlock()
		}
	}
}
