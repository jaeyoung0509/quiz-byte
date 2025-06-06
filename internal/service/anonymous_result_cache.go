package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quiz-byte/internal/cache" // Added for GenerateCacheKey
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"time"

	"go.uber.org/zap"
)

// const anonymousResultCachePrefix = "anonymous_result:" // Replaced by cache.GenerateCacheKey

// ErrAnonymousResultNotFound is returned when a cached result is not found.
var ErrAnonymousResultNotFound = errors.New("anonymous result not found in cache")

// AnonymousResultCacheService defines the interface for caching anonymous user quiz results.
type AnonymousResultCacheService interface {
	Put(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error
	Get(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error)
}

// anonymousResultCacheServiceImpl implements AnonymousResultCacheService using a generic cache.
type anonymousResultCacheServiceImpl struct {
	cache domain.Cache
	ttl   time.Duration
}

// NewAnonymousResultCacheService creates a new instance of anonymousResultCacheServiceImpl.
func NewAnonymousResultCacheService(cache domain.Cache, ttl time.Duration) AnonymousResultCacheService {
	if cache == nil {
		// Fallback to a no-op implementation if cache is nil to prevent panics
		logger.Get().Warn("AnonymousResultCacheService initialized with nil cache. Service will be no-op.")
		return &noopAnonymousResultCacheService{}
	}
	return &anonymousResultCacheServiceImpl{
		cache: cache,
		ttl:   ttl,
	}
}

func (s *anonymousResultCacheServiceImpl) generateKey(requestID string) string { // Renamed from cacheKey
	return cache.GenerateCacheKey("anonymous", "result", requestID) // Use central key generator
}

// Put stores the quiz result for an anonymous user in the cache.
func (s *anonymousResultCacheServiceImpl) Put(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
	if result == nil {
		return domain.NewInvalidInputError("cannot cache nil result")
	}

	key := s.generateKey(requestID)
	dataBytes, err := json.Marshal(result)
	if err != nil {
		logger.Get().Error("Failed to marshal anonymous result for caching", zap.Error(err), zap.String("requestID", requestID))
		return domain.NewInternalError("failed to marshal result for caching", err)
	}

	// domain.Cache interface expects string for Set value
	err = s.cache.Set(ctx, key, string(dataBytes), s.ttl)
	if err != nil {
		logger.Get().Error("Failed to cache anonymous result", zap.Error(err), zap.String("key", key))
		// Error from cache adapter is already wrapped, re-wrap into domain.InternalError
		return domain.NewInternalError(fmt.Sprintf("failed to set anonymous result to cache for key %s", key), err)
	}
	logger.Get().Debug("Successfully cached anonymous result", zap.String("key", key), zap.Duration("ttl", s.ttl))
	return nil
}

// Get retrieves the quiz result for an anonymous user from the cache.
func (s *anonymousResultCacheServiceImpl) Get(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error) {
	key := s.generateKey(requestID)
	// domain.Cache interface returns string
	dataString, err := s.cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrCacheMiss) {
			logger.Get().Debug("Anonymous result cache miss", zap.String("key", key))
			return nil, ErrAnonymousResultNotFound
		}
		logger.Get().Error("Failed to get anonymous result from cache", zap.Error(err), zap.String("key", key))
		// Error from cache adapter is already wrapped, re-wrap into domain.InternalError
		return nil, domain.NewInternalError(fmt.Sprintf("failed to get anonymous result from cache for key %s", key), err)
	}

	if dataString == "" { // Cache might return empty string, nil for a miss (depending on adapter)
		logger.Get().Debug("Anonymous result cache miss (empty data string)", zap.String("key", key))
		return nil, ErrAnonymousResultNotFound
	}

	var result dto.CheckAnswerResponse
	// Unmarshal from []byte(dataString)
	if err := json.Unmarshal([]byte(dataString), &result); err != nil {
		logger.Get().Error("Failed to unmarshal anonymous result from cache", zap.Error(err), zap.String("key", key))
		return nil, domain.NewInternalError(fmt.Sprintf("failed to unmarshal result from cache for key %s", key), err)
	}

	logger.Get().Debug("Successfully retrieved anonymous result from cache", zap.String("key", key))
	return &result, nil
}

// noopAnonymousResultCacheService is a no-op implementation for when caching is disabled or fails to initialize.
type noopAnonymousResultCacheService struct{}

func (s *noopAnonymousResultCacheService) Put(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
	logger.Get().Debug("No-op AnonymousResultCacheService: Put called", zap.String("requestID", requestID))
	return nil
}

func (s *noopAnonymousResultCacheService) Get(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error) {
	logger.Get().Debug("No-op AnonymousResultCacheService: Get called", zap.String("requestID", requestID))
	return nil, ErrAnonymousResultNotFound
}
