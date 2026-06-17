package authz

import (
	"fmt"
	"sync"
	"time"
)

type cacheEntry struct {
	decision  Decision
	expiresAt time.Time
}

type DecisionCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

func NewDecisionCache() *DecisionCache {
	return &DecisionCache{items: make(map[string]cacheEntry)}
}

func (c *DecisionCache) Get(subject, object, action string) (Decision, bool) {
	if c == nil {
		return Decision{}, false
	}
	key := cacheKey(subject, object, action)
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
		}
		return Decision{}, false
	}
	return entry.decision, true
}

func (c *DecisionCache) Set(subject, object, action string, decision Decision, ttl time.Duration) {
	if c == nil || ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cacheKey(subject, object, action)] = cacheEntry{decision: decision, expiresAt: time.Now().Add(ttl)}
}

func cacheKey(subject, object, action string) string {
	return fmt.Sprintf("%s\x00%s\x00%s", subject, object, action)
}
