package dao

import (
	"fmt"
	"sync"
	"time"
)

// BaseAWSObject implements the AWSObject interface with embedded fields.
type BaseAWSObject struct {
	ARN       string
	ID        string
	Name      string
	Region    string
	Tags      map[string]string
	CreatedAt *time.Time
	Raw       interface{} // Original AWS SDK object
}

// GetARN returns the Amazon Resource Name.
func (b *BaseAWSObject) GetARN() string {
	return b.ARN
}

// GetID returns the resource ID.
func (b *BaseAWSObject) GetID() string {
	return b.ID
}

// GetName returns the resource name.
func (b *BaseAWSObject) GetName() string {
	return b.Name
}

// GetRegion returns the AWS region.
func (b *BaseAWSObject) GetRegion() string {
	return b.Region
}

// GetTags returns the resource tags.
func (b *BaseAWSObject) GetTags() map[string]string {
	return b.Tags
}

// GetCreatedAt returns the creation timestamp.
func (b *BaseAWSObject) GetCreatedAt() *time.Time {
	return b.CreatedAt
}

// GetRaw returns the original AWS SDK object.
func (b *BaseAWSObject) GetRaw() interface{} {
	return b.Raw
}

// AWSResource is the base struct that all specific DAOs embed.
// It provides factory access, resource identification, and caching.
type AWSResource struct {
	Factory
	rid   *ResourceID
	cache *ResourceCache
	mx    sync.RWMutex
}

// Init initializes the AWSResource with factory and resource ID.
func (r *AWSResource) Init(f Factory, rid *ResourceID) {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.Factory = f
	r.rid = rid
}

// ResourceID returns the resource identifier.
func (r *AWSResource) ResourceID() *ResourceID {
	r.mx.RLock()
	defer r.mx.RUnlock()
	return r.rid
}

// getFactory returns the factory in a thread-safe manner.
func (r *AWSResource) getFactory() Factory {
	r.mx.RLock()
	defer r.mx.RUnlock()
	return r.Factory
}

// getCache returns the resource cache in a thread-safe manner.
func (r *AWSResource) getCache() *ResourceCache {
	r.mx.RLock()
	defer r.mx.RUnlock()
	return r.cache
}

// SetCache sets the resource cache (typically called during initialization).
func (r *AWSResource) SetCache(cache *ResourceCache) {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.cache = cache
}

// cacheKey generates a cache key from resource ID and region.
func (r *AWSResource) cacheKey(region string) string {
	r.mx.RLock()
	defer r.mx.RUnlock()
	if r.rid == nil {
		return region
	}
	return fmt.Sprintf("%s:%s", r.rid.String(), region)
}
