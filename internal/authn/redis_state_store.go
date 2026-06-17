package authn

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

type RedisStateStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStateStore(cfg config.RedisConfig) (*RedisStateStore, error) {
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
	return &RedisStateStore{client: client, prefix: prefix}, nil
}

func (s *RedisStateStore) Create(ctx context.Context, st LoginState, ttl time.Duration) error {
	if strings.TrimSpace(st.State) == "" {
		return ErrInvalidState
	}
	payload, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal login state: %w", err)
	}
	return s.client.Set(ctx, s.key(st.State), payload, ttl).Err()
}

func (s *RedisStateStore) Consume(ctx context.Context, state string) (LoginState, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return LoginState{}, ErrStateNotFound
	}
	key := s.key(state)
	payload, err := s.client.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return LoginState{}, ErrStateNotFound
		}
		return LoginState{}, err
	}
	var st LoginState
	if err := json.Unmarshal(payload, &st); err != nil {
		return LoginState{}, fmt.Errorf("unmarshal login state: %w", err)
	}
	return st, nil
}

func (s *RedisStateStore) key(state string) string {
	return fmt.Sprintf("%s:auth:state:%s", s.prefix, state)
}
