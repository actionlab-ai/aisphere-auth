package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(cfg config.RedisConfig) (*RedisStore, error) {
	addr := "127.0.0.1:6379"
	if len(cfg.Addrs) > 0 && strings.TrimSpace(cfg.Addrs[0]) != "" {
		addr = strings.TrimSpace(cfg.Addrs[0])
	}
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = "aisphere"
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &RedisStore{client: client, prefix: prefix}, nil
}

func (s *RedisStore) Create(ctx context.Context, sess *Session, ttl time.Duration) error {
	if sess == nil || sess.ID == "" {
		return errors.New("session id is required")
	}
	now := time.Now()
	copy := *sess
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = now
	}
	copy.UpdatedAt = now
	copy.ExpiresAt = now.Add(ttl)
	payload, err := json.Marshal(&copy)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.sessionKey(copy.ID), payload, ttl)
	if copy.Principal != nil && copy.Principal.SubjectID != "" {
		pipe.SAdd(ctx, s.subjectKey(copy.Principal.SubjectID), copy.ID)
		pipe.Expire(ctx, s.subjectKey(copy.Principal.SubjectID), ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, ErrNotFound
	}
	payload, err := s.client.Get(ctx, s.sessionKey(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(payload, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	if !sess.ExpiresAt.IsZero() && time.Now().After(sess.ExpiresAt) {
		_ = s.Delete(ctx, sessionID)
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *RedisStore) Update(ctx context.Context, sess *Session, ttl time.Duration) error {
	return s.Create(ctx, sess, ttl)
}

func (s *RedisStore) Delete(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	sess, _ := s.Get(ctx, sessionID)
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.sessionKey(sessionID))
	if sess != nil && sess.Principal != nil && sess.Principal.SubjectID != "" {
		pipe.SRem(ctx, s.subjectKey(sess.Principal.SubjectID), sessionID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) Touch(ctx context.Context, sessionID string, ttl time.Duration) error {
	sess, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	sess.UpdatedAt = time.Now()
	sess.ExpiresAt = time.Now().Add(ttl)
	payload, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.sessionKey(sessionID), payload, ttl)
	if sess.Principal != nil && sess.Principal.SubjectID != "" {
		pipe.Expire(ctx, s.subjectKey(sess.Principal.SubjectID), ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) DeleteBySubject(ctx context.Context, subjectID string) error {
	if strings.TrimSpace(subjectID) == "" {
		return nil
	}
	key := s.subjectKey(subjectID)
	ids, err := s.client.SMembers(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}
	pipe := s.client.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, s.sessionKey(id))
	}
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) sessionKey(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", s.prefix, sessionID)
}

func (s *RedisStore) subjectKey(subjectID string) string {
	return fmt.Sprintf("%s:session_subject:%s", s.prefix, subjectID)
}
