package server

import (
	"sync"
	"time"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
)

type tokenBucketLimiter struct {
	mu      sync.Mutex
	qps     float64
	burst   float64
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newTokenBucketLimiter(cfg config.InternalConfig) *tokenBucketLimiter {
	if !cfg.RateLimitEnabled {
		return nil
	}
	qps := cfg.RateLimitQPS
	if qps <= 0 {
		qps = 100
	}
	burst := cfg.RateLimitBurst
	if burst <= 0 {
		burst = qps * 2
	}
	return &tokenBucketLimiter{
		qps:     float64(qps),
		burst:   float64(burst),
		buckets: make(map[string]*tokenBucket),
	}
}

func (l *tokenBucketLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "anonymous"
	}

	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &tokenBucket{tokens: l.burst - 1, last: now}
		return true
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.qps
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--

	// Opportunistic cleanup to avoid unbounded bucket growth.
	if len(l.buckets) > 10000 {
		cutoff := now.Add(-10 * time.Minute)
		for k, item := range l.buckets {
			if item.last.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
	}
	return true
}
