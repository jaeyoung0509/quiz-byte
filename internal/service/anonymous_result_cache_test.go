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
	GetFunc    func(ctx context.Context, key string) (string, error)                        // Changed []byte to string
	SetFunc    func(ctx context.Context, key string, value string, ttl time.Duration) error // Changed value interface{} to string
	DeleteFunc func(ctx context.Context, key string) error
	// Add other methods if AnonymousResultCacheService uses them
	HGetFunc    func(ctx context.Context, key, field string) (string, error)
	HSetFunc    func(ctx context.Context, key string, field string, value string) error
	HGetAllFunc func(ctx context.Context, key string) (map[string]string, error)
	ExpireFunc  func(ctx context.Context, key string, expiration time.Duration) error
	PingFunc    func(ctx context.Context) error
}

func (m *ManualMockCache) Get(ctx context.Context, key string) (string, error) { // Changed []byte to string
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return "", errors.New("GetFunc not set")
}

func (m *ManualMockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error { // Changed value interface{} to string
	if m.SetFunc != nil {
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

// Adding other domain.Cache methods to satisfy the interface
func (m *ManualMockCache) HGet(ctx context.Context, key, field string) (string, error) {
	if m.HGetFunc != nil {
		return m.HGetFunc(ctx, key, field)
	}
	return "", errors.New("HGetFunc not set")
}
func (m *ManualMockCache) HSet(ctx context.Context, key string, field string, value string) error {
	if m.HSetFunc != nil {
		return m.HSetFunc(ctx, key, field, value)
	}
	return errors.New("HSetFunc not set")
}
func (m *ManualMockCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if m.HGetAllFunc != nil {
		return m.HGetAllFunc(ctx, key)
	}
	return nil, errors.New("HGetAllFunc not set")
}
func (m *ManualMockCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if m.ExpireFunc != nil {
		return m.ExpireFunc(ctx, key, expiration)
	}
	return errors.New("ExpireFunc not set")
}
func (m *ManualMockCache) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return errors.New("PingFunc not set")
}

func TestAnonymousResultCacheServiceImpl_Put(t *testing.T) {
	mockCache := &ManualMockCache{}
	ttl := 5 * time.Minute
	cacheService := service.NewAnonymousResultCacheService(mockCache, ttl)
	ctx := context.Background()

	requestID := "req123"
	result := &dto.CheckAnswerResponse{Score: 0.8, Explanation: "Good job!"}

	expectedKey := "quizbyte:anonymous:result:" + requestID
	expectedJSONData, _ := json.Marshal(result)    // This is []byte
	expectedStringData := string(expectedJSONData) // Convert to string for mock

	mockCache.SetFunc = func(ctx context.Context, key string, value string, duration time.Duration) error {
		assert.Equal(t, expectedKey, key)
		assert.Equal(t, expectedStringData, value) // Mock expects string
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
	expectedKey := "quizbyte:anonymous:result:" + requestID

	t.Run("Cache Hit", func(t *testing.T) {
		jsonDataBytes, _ := json.Marshal(expectedResult)
		jsonStringData := string(jsonDataBytes)                                     // Convert to string for mock
		mockCache.GetFunc = func(ctx context.Context, key string) (string, error) { // Mock returns string
			assert.Equal(t, expectedKey, key)
			return jsonStringData, nil
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("Cache Miss", func(t *testing.T) {
		mockCache.GetFunc = func(ctx context.Context, key string) (string, error) { // Mock returns string
			assert.Equal(t, expectedKey, key)
			return "", domain.ErrCacheMiss // Use the specific error from your domain package
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		assert.Equal(t, service.ErrAnonymousResultNotFound, err)
	})

	t.Run("Cache Miss (nil data, nil error - less common but possible)", func(t *testing.T) {
		mockCache.GetFunc = func(ctx context.Context, key string) (string, error) { // Mock returns string
			assert.Equal(t, expectedKey, key)
			return "", nil // Simulate a cache returning nil data and nil error for a miss
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		// The service implementation should consistently return ErrAnonymousResultNotFound for misses
		assert.Equal(t, service.ErrAnonymousResultNotFound, err)
	})

	t.Run("Cache Error", func(t *testing.T) {
		expectedErr := errors.New("some cache system error")
		mockCache.GetFunc = func(ctx context.Context, key string) (string, error) { // Mock returns string
			assert.Equal(t, expectedKey, key)
			return "", expectedErr
		}

		result, err := cacheService.Get(ctx, requestID)
		assert.Nil(t, result)
		assert.Error(t, err)
		// The service should wrap this into a domain.InternalError
		var domainErr *domain.DomainError
		assert.True(t, errors.As(err, &domainErr), "Error should be a domain.DomainError")
		if domainErr != nil {
			assert.Equal(t, domain.CodeInternal, domainErr.Code)
			assert.Contains(t, err.Error(), expectedErr.Error()) // Check original error message is contained
		}
	})

	t.Run("Deserialization Error", func(t *testing.T) {
		malformedJsonString := "{score:0.5, explanation:'bad json'"                 // missing quotes
		mockCache.GetFunc = func(ctx context.Context, key string) (string, error) { // Mock returns string
			assert.Equal(t, expectedKey, key)
			return malformedJsonString, nil
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
