package session

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("session not found")

const maxMemorySessions = 10000

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stop     chan struct{}
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{sessions: make(map[string]*Session), stop: make(chan struct{})}
	go s.cleanupLoop(time.Minute)
	return s
}

func (s *MemoryStore) Create(ctx context.Context, sess *Session, ttl time.Duration) error {
	_ = ctx
	if sess == nil || sess.ID == "" {
		return errors.New("session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.sessions) >= maxMemorySessions {
		s.cleanupExpiredLocked(now)
	}
	if len(s.sessions) >= maxMemorySessions {
		for key := range s.sessions {
			delete(s.sessions, key)
			break
		}
	}
	copy := *sess
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = now
	}
	copy.UpdatedAt = now
	copy.ExpiresAt = now.Add(ttl)
	s.sessions[copy.ID] = &copy
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	_ = ctx
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	if !ok || time.Now().After(sess.ExpiresAt) {
		s.mu.RUnlock()
		if ok {
			_ = s.Delete(ctx, sessionID)
		}
		return nil, ErrNotFound
	}
	copy := *sess
	s.mu.RUnlock()
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
	if subjectID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess != nil && sess.Principal != nil && sess.Principal.SubjectID == subjectID {
			delete(s.sessions, id)
		}
	}
	return nil
}

// CleanupExpired removes expired sessions. It is public for tests and also used
// by the background cleanup loop.
func (s *MemoryStore) CleanupExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanupExpiredLocked(now)
}

func (s *MemoryStore) cleanupExpiredLocked(now time.Time) int {
	removed := 0
	for id, sess := range s.sessions {
		if sess == nil || now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
			removed++
		}
	}
	return removed
}

func (s *MemoryStore) cleanupLoop(interval time.Duration) {
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
