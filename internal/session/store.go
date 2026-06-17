package session

import (
	"context"
	"time"
)

type Store interface {
	Create(ctx context.Context, sess *Session, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Update(ctx context.Context, sess *Session, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
	Touch(ctx context.Context, sessionID string, ttl time.Duration) error
	DeleteBySubject(ctx context.Context, subjectID string) error
}
