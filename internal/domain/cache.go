package domain

import (
	"context"
	"time"
)

// CacheError represents an error originating from the cache.
type CacheError string

func (e CacheError) Error() string {
	return string(e)
}

// ErrCacheMiss is returned when a key is not found in the cache.
const ErrCacheMiss = CacheError("cache: key not found")

// Cache defines the interface (port) for caching operations.
// Implementations of this interface will be the adapters (e.g., RedisCacheAdapter).
type Cache interface {
	// Get retrieves an item from the cache.
	// It returns ErrCacheMiss if the key is not found.
	Get(ctx context.Context, key string) (string, error)

	// Set adds an item to the cache, overwriting an existing item if one exists.
	// expiration is the duration for which the item should be cached.
	// If expiration is 0, the item is cached indefinitely (if supported by the adapter).
	Set(ctx context.Context, key string, value string, expiration time.Duration) error

	// Delete removes an item from the cache.
	// It should not return an error if the key is not found.
	Delete(ctx context.Context, key string) error

	// Ping checks the health of the cache service.
	Ping(ctx context.Context) error
}
