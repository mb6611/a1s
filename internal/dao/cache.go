package dao

import (
	"strings"
	"sync"
	"time"
)

// DefaultCacheTTL is the default time-to-live for cached DAO resources.
const DefaultCacheTTL = 5 * time.Second

// cacheEntry holds cached objects with their timestamp.
type cacheEntry struct {
	objects   []AWSObject
	timestamp time.Time
}

// ResourceCache provides TTL-based caching for DAO resources.
// This is separate from internal/aws/cache.go which caches AWS API responses.
type ResourceCache struct {
	data map[string]cacheEntry
	ttl  time.Duration
	mx   sync.RWMutex
}

// NewResourceCache creates a new ResourceCache with the specified TTL.
func NewResourceCache(ttl time.Duration) *ResourceCache {
	return &ResourceCache{
		data: make(map[string]cacheEntry),
		ttl:  ttl,
	}
}

// Get retrieves cached objects for the given key.
// Returns nil if the key is not found or the entry has expired.
func (c *ResourceCache) Get(key string) []AWSObject {
	c.mx.RLock()
	defer c.mx.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		return nil
	}

	if time.Since(entry.timestamp) > c.ttl {
		return nil
	}

	return entry.objects
}

// Set stores objects in the cache with the given key.
func (c *ResourceCache) Set(key string, objects []AWSObject) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.data[key] = cacheEntry{
		objects:   objects,
		timestamp: time.Now(),
	}
}

// Invalidate removes a specific key from the cache.
func (c *ResourceCache) Invalidate(key string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	delete(c.data, key)
}

// InvalidatePrefix removes all cache entries whose keys start with the given prefix.
func (c *ResourceCache) InvalidatePrefix(prefix string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			delete(c.data, key)
		}
	}
}

// Clear removes all entries from the cache.
func (c *ResourceCache) Clear() {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.data = make(map[string]cacheEntry)
}
