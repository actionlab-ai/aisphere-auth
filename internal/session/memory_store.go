package session

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("session not found")

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]*Session)}
}

func (s *MemoryStore) Create(ctx context.Context, sess *Session, ttl time.Duration) error {
	_ = ctx
	if sess == nil || sess.ID == "" {
		return errors.New("session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	copy := *sess
	copy.CreatedAt = now
	copy.UpdatedAt = now
	copy.ExpiresAt = now.Add(ttl)
	s.sessions[copy.ID] = &copy
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil, ErrNotFound
	}
	copy := *sess
	return &copy, nil
}

func (s *MemoryStore) Update(ctx context.Context, sess *Session, ttl time.Duration) error {
	return s.Create(ctx, sess, ttl)
}

func (s *MemoryStore) Delete(ctx context.Context, sessionID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *MemoryStore) Touch(ctx context.Context, sessionID string, ttl time.Duration) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	sess.UpdatedAt = now
	sess.ExpiresAt = now.Add(ttl)
	return nil
}

func (s *MemoryStore) DeleteBySubject(ctx context.Context, subjectID string) error {
	_ = ctx
	return nil
}
