package authn

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrStateNotFound = errors.New("login state not found")

type stateItem struct {
	state     LoginState
	expiresAt time.Time
}

type MemoryStateStore struct {
	mu    sync.RWMutex
	items map[string]stateItem
}

func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{items: map[string]stateItem{}}
}

func (s *MemoryStateStore) Create(ctx context.Context, st LoginState, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[st.State] = stateItem{state: st, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *MemoryStateStore) Consume(ctx context.Context, state string) (LoginState, error) {
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
