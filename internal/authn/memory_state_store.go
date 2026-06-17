package authn

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrStateNotFound  = errors.New("login state not found")
	ErrStateStoreFull = errors.New("login state store full")
)

const maxMemoryLoginStates = 10000

type stateItem struct {
	state     LoginState
	expiresAt time.Time
}

type MemoryStateStore struct {
	mu    sync.RWMutex
	items map[string]stateItem
	stop  chan struct{}
	once  sync.Once
}

func NewMemoryStateStore() *MemoryStateStore {
	s := &MemoryStateStore{items: map[string]stateItem{}, stop: make(chan struct{})}
	go s.cleanupLoop(time.Minute)
	return s
}

func (s *MemoryStateStore) Create(ctx context.Context, st LoginState, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(st.State) == 0 || len(st.State) > maxOAuthStateLength {
		return ErrInvalidState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= maxMemoryLoginStates {
		s.cleanupExpiredLocked(time.Now())
	}
	if len(s.items) >= maxMemoryLoginStates {
		return ErrStateStoreFull
	}
	s.items[st.State] = stateItem{state: st, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *MemoryStateStore) Consume(ctx context.Context, state string) (LoginState, error) {
	if err := ctx.Err(); err != nil {
		return LoginState{}, err
	}
	if len(state) == 0 || len(state) > maxOAuthStateLength {
		return LoginState{}, ErrStateNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[state]
	if !ok {
		return LoginState{}, ErrStateNotFound
	}
	delete(s.items, state)
	if time.Now().After(item.expiresAt) {
		return LoginState{}, ErrStateNotFound
	}
	return item.state, nil
}

func (s *MemoryStateStore) Close() error {
	s.once.Do(func() { close(s.stop) })
	return nil
}

// CleanupExpired removes expired login states. It is public for tests and also
// used by the background cleanup loop.
func (s *MemoryStateStore) CleanupExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanupExpiredLocked(now)
}

func (s *MemoryStateStore) cleanupExpiredLocked(now time.Time) int {
	removed := 0
	for key, item := range s.items {
		if now.After(item.expiresAt) {
			delete(s.items, key)
			removed++
		}
	}
	return removed
}

func (s *MemoryStateStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.CleanupExpired(time.Now())
		case <-s.stop:
			return
		}
	}
}
