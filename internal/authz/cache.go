package authz

import (
	"fmt"
	"sync"
	"time"
)

const defaultDecisionCacheMaxEntries = 10000

type cacheEntry struct {
	decision  Decision
	expiresAt time.Time
}

type DecisionCache struct {
	mu         sync.RWMutex
	items      map[string]cacheEntry
	maxEntries int
}

func NewDecisionCache() *DecisionCache {
	return &DecisionCache{items: make(map[string]cacheEntry), maxEntries: defaultDecisionCacheMaxEntries}
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
	now := time.Now()
	if c.maxEntries > 0 && len(c.items) >= c.maxEntries {
		c.cleanupExpiredLocked(now)
	}
	if c.maxEntries > 0 && len(c.items) >= c.maxEntries {
		c.evictOneLocked()
	}
	c.items[cacheKey(subject, object, action)] = cacheEntry{decision: decision, expiresAt: now.Add(ttl)}
}

func (c *DecisionCache) cleanupExpiredLocked(now time.Time) int {
	removed := 0
	for key, entry := range c.items {
		if now.After(entry.expiresAt) {
			delete(c.items, key)
			removed++
		}
	}
	return removed
}

func (c *DecisionCache) evictOneLocked() {
	var oldestKey string
	var oldest time.Time
	for key, entry := range c.items {
		if oldestKey == "" || entry.expiresAt.Before(oldest) {
			oldestKey = key
			oldest = entry.expiresAt
		}
	}
	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func cacheKey(subject, object, action string) string {
	return fmt.Sprintf("%s\x00%s\x00%s", subject, object, action)
}
