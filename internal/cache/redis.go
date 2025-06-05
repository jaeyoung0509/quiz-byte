package cache

import (
	"context"
	"fmt"
	"quiz-byte/internal/config"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and returns a new Redis client instance.
// It pings the server to ensure connectivity.
func NewRedisClient(redisCfg config.RedisConfig) (*redis.Client, error) {
	if redisCfg.Address == "" {
		return nil, fmt.Errorf("redis configuration is missing or address is empty")
	}

	opt := &redis.Options{
		Addr:     redisCfg.Address,
		Password: redisCfg.Password, // no password set
		DB:       redisCfg.DB,       // use default DB
	}

	// For more complex scenarios, redis.ParseURL might be useful
	// e.g., if the address was a URL like "redis://user:password@host:port/db"
	// For now, direct options are clear.

	client := redis.NewClient(opt)

	// Ping the Redis server to check the connection
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", redisCfg.Address, err)
	}

	return client, nil
}
