package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/service"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	// "go.uber.org/mock/gomock" // Not using generated mocks directly
)

// ManualMockCache for domain.Cache interface
type ManualMockCache struct {
	GetFunc    func(ctx context.Context, key string) ([]byte, error)
	SetFunc    func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	DeleteFunc func(ctx context.Context, key string) error
	// Add other methods if AnonymousResultCacheService uses them
}

func (m *ManualMockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return nil, errors.New("GetFunc not set")
}

func (m *ManualMockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.SetFunc != nil {
		// The service serializes to []byte before calling cache.Set
		// So, the mock should expect []byte if it's strict about type.
		// For this test, we'll assume the service passes []byte.
		return m.SetFunc(ctx, key, value, ttl)
	}
	return errors.New("SetFunc not set")
}

func (m *ManualMockCache) Delete(ctx context.Context, key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return errors.New("DeleteFunc not set")
}

func (m *ManualMockCache) GetKeysByPrefix(ctx context.Context, prefix string) ([]string, error) {
	panic("not implemented in mock")
}

func (m *ManualMockCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	panic("not implemented in mock")
}


func TestAnonymousResultCacheServiceImpl_Put(t *testing.T) {
	mockCache := &ManualMockCache{}
	ttl := 5 * time.Minute
	cacheService := service.NewAnonymousResultCacheService(mockCache, ttl)
	ctx := context.Background()

	requestID := "req123"
	result := &dto.CheckAnswerResponse{Score: 0.8, Explanation: "Good job!"}

	expectedKey := "anonymous_result:" + requestID
	expectedData, _ := json.Marshal(result)

	mockCache.SetFunc = func(ctx context.Context, key string, value interface{}, duration time.Duration) error {
		assert.Equal(t, expectedKey, key)
		assert.Equal(t, expectedData, value.([]byte)) // Service should pass []byte
		assert.Equal(t, ttl, duration)
		return nil
	}

	err := cacheService.Put(ctx, requestID, result)
	assert.NoError(t, err)
}

func TestAnonymousResultCacheServiceImpl_Get(t *testing.T) {
	mockCache := &ManualMockCache{}
	ttl := 5 * time.Minute
	cacheService := service.NewAnonymousResultCacheService(mockCache, ttl)
	ctx := context.Background()

	requestID := "req123"
	expectedResult := &dto.CheckAnswerResponse{Score: 0.9, Explanation: "Excellent!"}
	expectedKey := "anonymous_result:" + requestID

	t.Run("Cache Hit", func(t *testing.T) {
		jsonData, _ := json.Marshal(expectedResult)
		mockCache.GetFunc = func(ctx context.Context, key string) ([]byte, error) {
			assert.Equal(t, expectedKey, key)
			return jsonData, nil
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("Cache Miss", func(t *testing.T) {
		mockCache.GetFunc = func(ctx context.Context, key string) ([]byte, error) {
			assert.Equal(t, expectedKey, key)
			return nil, domain.ErrCacheMiss // Use the specific error from your domain package
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		assert.Equal(t, service.ErrAnonymousResultNotFound, err)
	})

	t.Run("Cache Miss (nil data, nil error - less common but possible)", func(t *testing.T) {
		mockCache.GetFunc = func(ctx context.Context, key string) ([]byte, error) {
			assert.Equal(t, expectedKey, key)
			return nil, nil // Simulate a cache returning nil data and nil error for a miss
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		// The service implementation should consistently return ErrAnonymousResultNotFound for misses
		assert.Equal(t, service.ErrAnonymousResultNotFound, err)
	})


	t.Run("Cache Error", func(t *testing.T) {
		expectedErr := errors.New("some cache system error")
		mockCache.GetFunc = func(ctx context.Context, key string) ([]byte, error) {
			assert.Equal(t, expectedKey, key)
			return nil, expectedErr
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedErr.Error())
	})

	t.Run("Deserialization Error", func(t *testing.T) {
		malformedJsonData := []byte("{score:0.5, explanation:'bad json'") // missing quotes
		mockCache.GetFunc = func(ctx context.Context, key string) ([]byte, error) {
			assert.Equal(t, expectedKey, key)
			return malformedJsonData, nil
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal result")
	})
}

func TestNewAnonymousResultCacheService_NilCache(t *testing.T) {
	ttl := 5 * time.Minute
	// Initialize with nil cache
	cacheService := service.NewAnonymousResultCacheService(nil, ttl)
	ctx := context.Background()

	requestID := "reqNoCache"
	resultToPut := &dto.CheckAnswerResponse{Score: 0.7, Explanation: "Testing no-op"}

	// Test Put on no-op service
	errPut := cacheService.Put(ctx, requestID, resultToPut)
	assert.NoError(t, errPut, "Put on no-op service should not error")

	// Test Get on no-op service
	retrievedResult, errGet := cacheService.Get(ctx, requestID)
	assert.Nil(t, retrievedResult, "Get on no-op service should return nil result")
	assert.Equal(t, service.ErrAnonymousResultNotFound, errGet, "Get on no-op service should return ErrAnonymousResultNotFound")
}
