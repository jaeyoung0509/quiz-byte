package adapter

import (
	"context"
	"fmt"
	"quiz-byte/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCacheAdapter implements the domain.Cache interface using a Redis client.
type RedisCacheAdapter struct {
	client *redis.Client
}

// NewRedisCacheAdapter creates a new instance of RedisCacheAdapter.
// It expects a connected *redis.Client.
func NewRedisCacheAdapter(client *redis.Client) domain.Cache {
	return &RedisCacheAdapter{client: client}
}

// Get retrieves an item from the Redis cache.
// It translates redis.Nil to domain.ErrCacheMiss.
func (r *RedisCacheAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", domain.ErrCacheMiss
		}
		return "", fmt.Errorf("redis Get failed for key %s: %w", key, err)
	}
	return val, nil
}

// Set adds an item to the Redis cache.
func (r *RedisCacheAdapter) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	if err := r.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("redis Set failed for key %s: %w", key, err)
	}
	return nil
}

// Delete removes an item from the Redis cache.
func (r *RedisCacheAdapter) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis Del failed for key %s: %w", key, err)
	}
	return nil
}

// Ping checks the health of the Redis server.
func (r *RedisCacheAdapter) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis Ping failed: %w", err)
	}
	return nil
}

// HGet implements Cache.HGet
func (r *RedisCacheAdapter) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := r.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
			return "", domain.ErrCacheMiss
		}
		return "", fmt.Errorf("redis HGet failed for key %s, field %s: %w", key, field, err)
	}
	return val, nil
}

// HGetAll implements Cache.HGetAll
func (r *RedisCacheAdapter) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	val, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrCacheMiss
		}
		return nil, fmt.Errorf("redis HGetAll failed for key %s: %w", key, err)
	}
	return val, nil
}

// HSet implements Cache.HSet
func (r *RedisCacheAdapter) HSet(ctx context.Context, key string, field string, value string) error {
	if err := r.client.HSet(ctx, key, field, value).Err(); err != nil {
		return fmt.Errorf("redis HSet failed for key %s, field %s: %w", key, field, err)
	}
	return nil
}

// Expire implements Cache.Expire
func (r *RedisCacheAdapter) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if err := r.client.Expire(ctx, key, expiration).Err(); err != nil {
		return fmt.Errorf("redis Expire failed for key %s: %w", key, err)
	}
	return nil
}
