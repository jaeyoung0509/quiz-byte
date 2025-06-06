package service

import (
	"bytes"        // Added for gob
	"context"      // Keep one context
	"encoding/gob" // Added for gob
	"fmt"
	"testing"
	"time"

	"quiz-byte/internal/cache" // Added for GenerateCacheKey
	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementation of the cache interface for AnswerCacheService

// --- Mocks for AnswerCacheService Tests ---

type MockAnswerCacheDomainCache struct {
	mock.Mock
}

func (m *MockAnswerCacheDomainCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockAnswerCacheDomainCache) HSet(ctx context.Context, key string, field string, value string) error {
	args := m.Called(ctx, key, field, value)
	return args.Error(0)
}

func (m *MockAnswerCacheDomainCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.Called(ctx, key, expiration)
	return args.Error(0)
}

// Get, Set, Delete, Ping, HGet are not used by AnswerCacheService, so not mocked here.
// Adding missing methods to satisfy domain.Cache interface
func (m *MockAnswerCacheDomainCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockAnswerCacheDomainCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockAnswerCacheDomainCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockAnswerCacheDomainCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAnswerCacheDomainCache) HGet(ctx context.Context, key, field string) (string, error) {
	args := m.Called(ctx, key, field)
	return args.String(0), args.Error(1)
}

// Ensure MockAnswerCacheDomainCache implements domain.Cache
var _ domain.Cache = (*MockAnswerCacheDomainCache)(nil)

// MockAnswerCacheQuizRepository is removed, will use MockQuizRepository from mocks_test.go

// --- Tests for AnswerCacheService ---

func TestAnswerCacheServiceImpl_GetAnswerFromCache(t *testing.T) {
	ctx := context.Background()
	baseQuizID := "quizGet123"
	baseUserAnswerText := "test user answer for get"
	baseUserAnswerEmbedding := []float32{0.1, 0.2, 0.3}

	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			SimilarityThreshold: 0.9,
		},
	}

	t.Run("Cache Hit - High Similarity", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository)                                                             // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour) // Example default
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)

		// Cache key for the HASH where evaluations are stored
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		similarEmbedding := []float32{0.11, 0.21, 0.31} // Similar
		expectedEvalResponse := &dto.CheckAnswerResponse{Score: 0.85, Explanation: "Cached explanation", ModelAnswer: "Original Model Ans"}

		// UserAnswer in CachedAnswerEvaluation is used for integrity check if present, and for logging.
		// The field key for HGetAll is the hashed version of an original user answer.
		// Let's assume "similar cached text" was the original user answer that got hashed to produce a field key.
		userAnswerForFieldKey := "similar cached text"
		fieldKey := hashString(userAnswerForFieldKey)

		cachedData := CachedAnswerEvaluation{
			Evaluation: expectedEvalResponse,
			Embedding:  similarEmbedding,
			UserAnswer: userAnswerForFieldKey, // Store original text that matches the fieldKey's hash
		}

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(cachedData)
		gobEncodedCachedData := expectedBuffer.String()

		cacheReturnMap := map[string]string{fieldKey: gobEncodedCachedData}

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(cacheReturnMap, nil).Once()
		updatedModelAnswer := "Updated Model Answer For Cache Hit"
		mockRepo.On("GetQuizByID", ctx, baseQuizID).Return(&domain.Quiz{ModelAnswers: []string{updatedModelAnswer}}, nil).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, expectedEvalResponse.Score, response.Score)
		assert.Equal(t, updatedModelAnswer, response.ModelAnswer) // Check model answer update
		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Cache Hit - Low Similarity", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)

		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)
		dissimilarEmbedding := []float32{0.9, 0.8, 0.7} // Dissimilar
		cachedEvalResponse := &dto.CheckAnswerResponse{Score: 0.30, Explanation: "Dissimilar cached explanation"}

		userAnswerForFieldKey := "dissimilar cached text"
		fieldKey := hashString(userAnswerForFieldKey)

		cachedData := CachedAnswerEvaluation{
			Evaluation: cachedEvalResponse,
			Embedding:  dissimilarEmbedding,
			UserAnswer: userAnswerForFieldKey,
		}

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(cachedData)
		gobEncodedCachedData := expectedBuffer.String()

		cacheReturnMap := map[string]string{fieldKey: gobEncodedCachedData}

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(cacheReturnMap, nil).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)

		assert.NoError(t, err)
		assert.Nil(t, response) // Cache miss due to low similarity
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Cache Miss - Empty Cache", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(map[string]string{}, nil).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Cache Miss - CacheKey Not Found (ErrCacheMiss)", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(nil, domain.ErrCacheMiss).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Cache Error on HGetAll", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)
		expectedError := fmt.Errorf("HGetAll failed")

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(nil, expectedError).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.ErrorIs(t, err, expectedError)
		assert.Nil(t, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Unmarshal Error for a cached entry", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		// One valid, one invalid
		validUserAnswer := "valid answer text"
		validFieldKey := hashString(validUserAnswer)
		validCachedData := CachedAnswerEvaluation{
			Evaluation: &dto.CheckAnswerResponse{Score: 0.9},
			Embedding:  []float32{0.11, 0.21, 0.31}, // Assuming this is similar to baseUserAnswerEmbedding
			UserAnswer: validUserAnswer,
		}
		var validBuffer bytes.Buffer
		_ = gob.NewEncoder(&validBuffer).Encode(validCachedData)
		gobEncodedValidData := validBuffer.String()

		cacheReturnMap := map[string]string{
			validFieldKey:     gobEncodedValidData,
			"invalidFieldKey": "this is not gob", // This will cause decode error
		}
		// Mock GetQuizByID for the valid entry if it's found and similar
		mockRepo.On("GetQuizByID", ctx, baseQuizID).Return(&domain.Quiz{ModelAnswers: []string{"Updated"}}, nil).Maybe()

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(cacheReturnMap, nil).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)

		// Depending on the order of iteration, it might find the valid one or skip the invalid one.
		// If it finds the valid one and it's similar, it returns it.
		// If the valid one is not similar enough, it returns nil, nil after trying all.
		// The key is that an unmarshal error for one entry doesn't stop processing of others.
		assert.NoError(t, err)
		// If baseUserAnswerEmbedding is similar to validCachedData.Embedding, then response will be non-nil.
		// For this specific test, let's assume it is similar.
		assert.NotNil(t, response)
		assert.Equal(t, validCachedData.Evaluation.Score, response.Score)

		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t) // GetQuizByID should be called if the valid entry is hit
	})

	t.Run("GetQuizByID Fails During Model Answer Update", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)

		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)
		similarEmbedding := []float32{0.11, 0.21, 0.31}
		originalModelAnswer := "Original Model From Cache"
		expectedEvalResponse := &dto.CheckAnswerResponse{Score: 0.85, Explanation: "Cached explanation", ModelAnswer: originalModelAnswer}

		userAnswerForFieldKey := "somekey answer" // Text that would produce "somekey" field
		fieldKey := hashString(userAnswerForFieldKey)

		cachedData := CachedAnswerEvaluation{
			Evaluation: expectedEvalResponse,
			Embedding:  similarEmbedding,
			UserAnswer: userAnswerForFieldKey,
		}
		var buffer bytes.Buffer
		_ = gob.NewEncoder(&buffer).Encode(cachedData)
		gobEncodedData := buffer.String()
		cacheReturnMap := map[string]string{fieldKey: gobEncodedData}

		mockCache.On("HGetAll", ctx, hashCacheKey).Return(cacheReturnMap, nil).Once()
		repoError := fmt.Errorf("repo GetQuizByID failed")
		mockRepo.On("GetQuizByID", ctx, baseQuizID).Return(nil, repoError).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)

		assert.NoError(t, err) // The error from GetQuizByID is logged but not returned to QuizService
		assert.NotNil(t, response)
		assert.Equal(t, expectedEvalResponse.Score, response.Score)
		assert.Equal(t, originalModelAnswer, response.ModelAnswer) // ModelAnswer remains original
		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Cache Service disabled (nil cache)", func(t *testing.T) {
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository // Repo might still be non-nil
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(nil, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold) // nil cache

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Empty User Answer Embedding", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockQuizRepository) // Changed to MockQuizRepository
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 1*time.Hour)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, mockRepo, answerEvaluationTTL, embeddingSimilarityThreshold)

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, []float32{}, baseUserAnswerText) // Empty embedding
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockCache.AssertNotCalled(t, "HGetAll")
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})
}

func TestAnswerCacheServiceImpl_PutAnswerToCache(t *testing.T) {
	ctx := context.Background()
	baseQuizID := "quizPut123"
	baseUserAnswerText := "test user answer for put"
	baseUserAnswerEmbedding := []float32{0.3, 0.2, 0.1}
	baseEvaluation := &dto.CheckAnswerResponse{
		Score:       0.9,
		Explanation: "Great answer!",
		ModelAnswer: "Model answer for put",
	}
	testAnswerEvaluationTTLString := "30m"
	testAnswerEvaluationTTL, _ := time.ParseDuration(testAnswerEvaluationTTLString)
	cfg := &config.Config{
		CacheTTLs: config.CacheTTLConfig{
			AnswerEvaluation: testAnswerEvaluationTTLString,
		},
	}

	t.Run("Successful Cache Write", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		// mockRepo is not used by PutAnswerToCache
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, testAnswerEvaluationTTL)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, nil, answerEvaluationTTL, embeddingSimilarityThreshold)

		expectedCachedEval := CachedAnswerEvaluation{
			Evaluation: baseEvaluation,
			Embedding:  baseUserAnswerEmbedding,
			UserAnswer: baseUserAnswerText,
		}
		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedCachedEval)
		expectedGobData := expectedBuffer.String()

		fieldKey := hashString(baseUserAnswerText)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		mockCache.On("HSet", ctx, hashCacheKey, fieldKey, expectedGobData).Return(nil).Once()
		expectedTTL, _ := time.ParseDuration(testAnswerEvaluationTTLString)
		mockCache.On("Expire", ctx, hashCacheKey, expectedTTL).Return(nil).Once()

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.NoError(t, err)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache HSet Fails", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, testAnswerEvaluationTTL)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, nil, answerEvaluationTTL, embeddingSimilarityThreshold)
		// cacheKey variable is not used directly here for HSet error, hashCacheKey is
		expectedError := fmt.Errorf("HSet failed")

		expectedCachedEval := CachedAnswerEvaluation{
			Evaluation: baseEvaluation,
			Embedding:  baseUserAnswerEmbedding,
			UserAnswer: baseUserAnswerText,
		}
		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedCachedEval)
		expectedGobData := expectedBuffer.String()

		fieldKey := hashString(baseUserAnswerText)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		mockCache.On("HSet", ctx, hashCacheKey, fieldKey, expectedGobData).Return(expectedError).Once()

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.ErrorIs(t, err, expectedError)
		mockCache.AssertExpectations(t)
		mockCache.AssertNotCalled(t, "Expire") // Expire should not be called if HSet fails
	})

	t.Run("Cache Expire Fails", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, testAnswerEvaluationTTL)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, nil, answerEvaluationTTL, embeddingSimilarityThreshold)
		// cacheKey variable is not used directly here for Expire error, hashCacheKey is
		expectedError := fmt.Errorf("Expire failed")

		expectedCachedEval := CachedAnswerEvaluation{
			Evaluation: baseEvaluation,
			Embedding:  baseUserAnswerEmbedding,
			UserAnswer: baseUserAnswerText,
		}
		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedCachedEval)
		expectedGobData := expectedBuffer.String()

		fieldKey := hashString(baseUserAnswerText)
		hashCacheKey := cache.GenerateCacheKey("answer", "evaluation_map", baseQuizID)

		mockCache.On("HSet", ctx, hashCacheKey, fieldKey, expectedGobData).Return(nil).Once()
		expectedTTL, _ := time.ParseDuration(testAnswerEvaluationTTLString)
		mockCache.On("Expire", ctx, hashCacheKey, expectedTTL).Return(expectedError).Once()

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.ErrorIs(t, err, expectedError)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache Service disabled (nil cache)", func(t *testing.T) {
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, testAnswerEvaluationTTL)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(nil, nil, answerEvaluationTTL, embeddingSimilarityThreshold) // nil cache

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.NoError(t, err) // Should not error, just skip
	})

	t.Run("Empty User Answer Embedding", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, testAnswerEvaluationTTL)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, nil, answerEvaluationTTL, embeddingSimilarityThreshold)

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, []float32{}, baseEvaluation) // Empty embedding
		assert.NoError(t, err)                                                                            // Skips caching
		mockCache.AssertNotCalled(t, "HSet")
		mockCache.AssertNotCalled(t, "Expire")
	})

	t.Run("Nil Evaluation", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, testAnswerEvaluationTTL)
		embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
		service := NewAnswerCacheService(mockCache, nil, answerEvaluationTTL, embeddingSimilarityThreshold)

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, nil) // Nil evaluation
		assert.NoError(t, err)                                                                             // Skips caching
		mockCache.AssertNotCalled(t, "HSet")
		mockCache.AssertNotCalled(t, "Expire")
	})
}
