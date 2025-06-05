package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"time"

	"go.uber.org/zap"
)

const anonymousResultCachePrefix = "anonymous_result:"

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

func (s *anonymousResultCacheServiceImpl) cacheKey(requestID string) string {
	return anonymousResultCachePrefix + requestID
}

// Put stores the quiz result for an anonymous user in the cache.
func (s *anonymousResultCacheServiceImpl) Put(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
	if result == nil {
		return errors.New("cannot cache nil result")
	}

	key := s.cacheKey(requestID)
	data, err := json.Marshal(result)
	if err != nil {
		logger.Get().Error("Failed to marshal anonymous result for caching", zap.Error(err), zap.String("requestID", requestID))
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	err = s.cache.Set(ctx, key, data, s.ttl)
	if err != nil {
		logger.Get().Error("Failed to cache anonymous result", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("failed to set cache: %w", err)
	}
	logger.Get().Debug("Successfully cached anonymous result", zap.String("key", key), zap.Duration("ttl", s.ttl))
	return nil
}

// Get retrieves the quiz result for an anonymous user from the cache.
func (s *anonymousResultCacheServiceImpl) Get(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error) {
	key := s.cacheKey(requestID)
	data, err := s.cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrCacheMiss) { // Assuming domain.Cache returns a specific error for cache miss
			logger.Get().Debug("Anonymous result cache miss", zap.String("key", key))
			return nil, ErrAnonymousResultNotFound // Return specific error for not found
		}
		logger.Get().Error("Failed to get anonymous result from cache", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	if data == nil { // Should be handled by domain.ErrCacheMiss, but as a safeguard
		logger.Get().Debug("Anonymous result cache miss (nil data)", zap.String("key", key))
		return nil, ErrAnonymousResultNotFound
	}

	var result dto.CheckAnswerResponse
	if err := json.Unmarshal(data, &result); err != nil {
		logger.Get().Error("Failed to unmarshal anonymous result from cache", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
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
