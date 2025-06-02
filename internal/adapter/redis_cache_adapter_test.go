package adapter

import (
	"context"
	"errors"
	"quiz-byte/internal/domain"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisCacheAdapter_Get(t *testing.T) {
	db, mock := redismock.NewClientMock()
	adapter := NewRedisCacheAdapter(db)
	ctx := context.Background()

	key := "testkey"
	expectedValue := "testvalue"

	t.Run("Success", func(t *testing.T) {
		mock.ExpectGet(key).SetVal(expectedValue)
		val, err := adapter.Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, expectedValue, val)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CacheMiss", func(t *testing.T) {
		mock.ExpectGet(key).SetErr(redis.Nil)
		val, err := adapter.Get(ctx, key)
		assert.ErrorIs(t, err, domain.ErrCacheMiss)
		assert.Empty(t, val)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RedisError", func(t *testing.T) {
		redisErr := errors.New("some redis error")
		mock.ExpectGet(key).SetErr(redisErr)
		val, err := adapter.Get(ctx, key)
		assert.ErrorIs(t, err, redisErr)
		assert.Empty(t, val)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRedisCacheAdapter_Set(t *testing.T) {
	db, mock := redismock.NewClientMock()
	adapter := NewRedisCacheAdapter(db)
	ctx := context.Background()

	key := "testkey"
	value := "testvalue"
	expiration := 1 * time.Hour

	t.Run("Success", func(t *testing.T) {
		mock.ExpectSet(key, value, expiration).SetVal("OK")
		err := adapter.Set(ctx, key, value, expiration)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RedisError", func(t *testing.T) {
		redisErr := errors.New("some redis error")
		mock.ExpectSet(key, value, expiration).SetErr(redisErr)
		err := adapter.Set(ctx, key, value, expiration)
		assert.ErrorIs(t, err, redisErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRedisCacheAdapter_Delete(t *testing.T) {
	db, mock := redismock.NewClientMock()
	adapter := NewRedisCacheAdapter(db)
	ctx := context.Background()

	key := "testkey"

	t.Run("Success", func(t *testing.T) {
		mock.ExpectDel(key).SetVal(1) // 1 key deleted
		err := adapter.Delete(ctx, key)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SuccessKeyNotFound", func(t *testing.T) {
		mock.ExpectDel(key).SetVal(0) // 0 keys deleted
		err := adapter.Delete(ctx, key)
		assert.NoError(t, err) // Delete should not error if key not found
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RedisError", func(t *testing.T) {
		redisErr := errors.New("some redis error")
		mock.ExpectDel(key).SetErr(redisErr)
		err := adapter.Delete(ctx, key)
		assert.ErrorIs(t, err, redisErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRedisCacheAdapter_Ping(t *testing.T) {
	db, mock := redismock.NewClientMock()
	adapter := NewRedisCacheAdapter(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectPing().SetVal("PONG")
		err := adapter.Ping(ctx)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RedisError", func(t *testing.T) {
		redisErr := errors.New("some redis error")
		mock.ExpectPing().SetErr(redisErr)
		err := adapter.Ping(ctx)
		assert.ErrorIs(t, err, redisErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
