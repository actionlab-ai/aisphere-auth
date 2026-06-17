package session

import (
	"context"
	"time"
)

// Store implementations must return a copy of Session from Get. Callers may
// mutate the returned value, so stores must not expose shared in-memory pointers.
type Store interface {
	Create(ctx context.Context, sess *Session, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Update(ctx context.Context, sess *Session, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
	Touch(ctx context.Context, sessionID string, ttl time.Duration) error
	DeleteBySubject(ctx context.Context, subjectID string) error
	Ping(ctx context.Context) error
	Close() error
}
