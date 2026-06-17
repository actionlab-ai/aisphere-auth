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

var redisDeleteSessionScript = redis.NewScript(`
local payload = redis.call("GET", KEYS[1])
if payload then
  local ok, sess = pcall(cjson.decode, payload)
  if ok and sess and sess["Principal"] then
    local principal = sess["Principal"]
    local subject = principal["subjectId"] or principal["SubjectID"]
    if subject and subject ~= "" then
      redis.call("SREM", ARGV[1] .. ":session_subject:" .. subject, ARGV[2])
    end
  end
end
redis.call("DEL", KEYS[1])
return 1
`)

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
	store := &RedisStore{client: client, prefix: prefix}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return store, nil
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
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	_, err := redisDeleteSessionScript.Run(ctx, s.client, []string{s.sessionKey(sessionID)}, s.prefix, sessionID).Result()
	return err
}

func (s *RedisStore) Touch(ctx context.Context, sessionID string, ttl time.Duration) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrNotFound
	}
	exists, err := s.client.Expire(ctx, s.sessionKey(sessionID), ttl).Result()
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
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

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) sessionKey(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", s.prefix, sessionID)
}

func (s *RedisStore) subjectKey(subjectID string) string {
	return fmt.Sprintf("%s:session_subject:%s", s.prefix, subjectID)
}
