package aws

import (
	"sync"
	"time"
)

type CacheEntry struct {
	Value     interface{}
	Timestamp time.Time
	TTL       time.Duration
	RefreshAt time.Time
}

type CacheConfig struct {
	DefaultTTL time.Duration
	MaxEntries int
}

type ResourceCache struct {
	entries    map[string]*CacheEntry
	defaultTTL time.Duration
	maxEntries int
	mx         sync.RWMutex
}

func NewResourceCache(cfg *CacheConfig) *ResourceCache {
	defaultTTL := 5 * time.Second
	maxEntries := 1000

	if cfg != nil {
		if cfg.DefaultTTL > 0 {
			defaultTTL = cfg.DefaultTTL
		}
		if cfg.MaxEntries > 0 {
			maxEntries = cfg.MaxEntries
		}
	}

	return &ResourceCache{
		entries:    make(map[string]*CacheEntry),
		defaultTTL: defaultTTL,
		maxEntries: maxEntries,
	}
}

func (c *ResourceCache) Get(key string) (interface{}, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.RefreshAt) {
		return nil, false
	}

	return entry.Value, true
}

func (c *ResourceCache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

func (c *ResourceCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mx.Lock()
	defer c.mx.Unlock()

	// Evict oldest entries if at capacity
	if len(c.entries) >= c.maxEntries {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, v := range c.entries {
			if first || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
				first = false
			}
		}
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}

	now := time.Now()
	c.entries[key] = &CacheEntry{
		Value:     value,
		Timestamp: now,
		TTL:       ttl,
		RefreshAt: now.Add(ttl),
	}
}

func (c *ResourceCache) Delete(key string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	delete(c.entries, key)
}

func (c *ResourceCache) DeletePrefix(prefix string) int {
	c.mx.Lock()
	defer c.mx.Unlock()

	if prefix == "" {
		return 0
	}

	count := 0
	for key := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.entries, key)
			count++
		}
	}

	return count
}

func (c *ResourceCache) Invalidate() {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

func (c *ResourceCache) Clear() {
	c.Invalidate()
}
