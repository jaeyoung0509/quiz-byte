package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	// util is needed for CosineSimilarity, but it's called by the service, not directly by the test usually.
	// "quiz-byte/internal/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMain for logger initialization
func TestMain(m *testing.M) {
	cfg := &config.Config{}
	if err := logger.Initialize(cfg); err != nil {
		panic("Failed to initialize logger for tests: " + err.Error())
	}
	exitVal := m.Run()
	_ = logger.Sync()
	os.Exit(exitVal)
}

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

type MockAnswerCacheQuizRepository struct {
	mock.Mock
}

func (m *MockAnswerCacheQuizRepository) GetQuizByID(id string) (*domain.Quiz, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

// Other repository methods are not used by AnswerCacheService.

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
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)

		cacheKey := AnswerCachePrefix + baseQuizID
		similarEmbedding := []float32{0.11, 0.21, 0.31} // Similar
		expectedEvalResponse := &dto.CheckAnswerResponse{Score: 0.85, Explanation: "Cached explanation", ModelAnswer: "Original Model Ans"}
		cachedData := CachedAnswerEvaluation{
			Evaluation: expectedEvalResponse,
			Embedding:  similarEmbedding,
			UserAnswer: "similar cached text",
		}
		marshaledCachedData, _ := json.Marshal(cachedData)
		cacheReturnMap := map[string]string{cachedData.UserAnswer: string(marshaledCachedData)}

		mockCache.On("HGetAll", ctx, cacheKey).Return(cacheReturnMap, nil).Once()
		updatedModelAnswer := "Updated Model Answer For Cache Hit"
		mockRepo.On("GetQuizByID", baseQuizID).Return(&domain.Quiz{ModelAnswers: []string{updatedModelAnswer}}, nil).Once()

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
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)

		cacheKey := AnswerCachePrefix + baseQuizID
		dissimilarEmbedding := []float32{0.9, 0.8, 0.7} // Dissimilar
		cachedEvalResponse := &dto.CheckAnswerResponse{Score: 0.30, Explanation: "Dissimilar cached explanation"}
		cachedData := CachedAnswerEvaluation{
			Evaluation: cachedEvalResponse,
			Embedding:  dissimilarEmbedding,
			UserAnswer: "dissimilar cached text",
		}
		marshaledCachedData, _ := json.Marshal(cachedData)
		cacheReturnMap := map[string]string{cachedData.UserAnswer: string(marshaledCachedData)}

		mockCache.On("HGetAll", ctx, cacheKey).Return(cacheReturnMap, nil).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)

		assert.NoError(t, err)
		assert.Nil(t, response) // Cache miss due to low similarity
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Cache Miss - Empty Cache", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)
		cacheKey := AnswerCachePrefix + baseQuizID

		mockCache.On("HGetAll", ctx, cacheKey).Return(map[string]string{}, nil).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Cache Miss - CacheKey Not Found (ErrCacheMiss)", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)
		cacheKey := AnswerCachePrefix + baseQuizID

		mockCache.On("HGetAll", ctx, cacheKey).Return(nil, domain.ErrCacheMiss).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})


	t.Run("Cache Error on HGetAll", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)
		cacheKey := AnswerCachePrefix + baseQuizID
		expectedError := fmt.Errorf("HGetAll failed")

		mockCache.On("HGetAll", ctx, cacheKey).Return(nil, expectedError).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.ErrorIs(t, err, expectedError)
		assert.Nil(t, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Unmarshal Error for a cached entry", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)
		cacheKey := AnswerCachePrefix + baseQuizID

		// One valid, one invalid
		validCachedData := CachedAnswerEvaluation{Evaluation: &dto.CheckAnswerResponse{Score: 0.9}, Embedding: []float32{0.11,0.21,0.31}}
		marshaledValid, _ := json.Marshal(validCachedData)
		cacheReturnMap := map[string]string{
			"validEntry":   string(marshaledValid),
			"invalidEntry": "this is not json",
		}
		// Mock GetQuizByID for the valid entry if it's found and similar
		mockRepo.On("GetQuizByID", baseQuizID).Return(&domain.Quiz{ModelAnswers: []string{"Updated"}}, nil).Maybe()


		mockCache.On("HGetAll", ctx, cacheKey).Return(cacheReturnMap, nil).Once()

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
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)

		cacheKey := AnswerCachePrefix + baseQuizID
		similarEmbedding := []float32{0.11, 0.21, 0.31}
		originalModelAnswer := "Original Model From Cache"
		expectedEvalResponse := &dto.CheckAnswerResponse{Score: 0.85, Explanation: "Cached explanation", ModelAnswer: originalModelAnswer}
		cachedData := CachedAnswerEvaluation{
			Evaluation: expectedEvalResponse,
			Embedding:  similarEmbedding,
		}
		marshaledCachedData, _ := json.Marshal(cachedData)
		cacheReturnMap := map[string]string{"somekey": string(marshaledCachedData)}

		mockCache.On("HGetAll", ctx, cacheKey).Return(cacheReturnMap, nil).Once()
		repoError := fmt.Errorf("repo GetQuizByID failed")
		mockRepo.On("GetQuizByID", baseQuizID).Return(nil, repoError).Once()

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)

		assert.NoError(t, err) // The error from GetQuizByID is logged but not returned to QuizService
		assert.NotNil(t, response)
		assert.Equal(t, expectedEvalResponse.Score, response.Score)
		assert.Equal(t, originalModelAnswer, response.ModelAnswer) // ModelAnswer remains original
		mockCache.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Cache Service disabled (nil cache)", func(t *testing.T) {
		mockRepo := new(MockAnswerCacheQuizRepository) // Repo might still be non-nil
		service := NewAnswerCacheService(nil, mockRepo, cfg) // nil cache

		response, err := service.GetAnswerFromCache(ctx, baseQuizID, baseUserAnswerEmbedding, baseUserAnswerText)
		assert.NoError(t, err)
		assert.Nil(t, response)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
	})

	t.Run("Empty User Answer Embedding", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		mockRepo := new(MockAnswerCacheQuizRepository)
		service := NewAnswerCacheService(mockCache, mockRepo, cfg)

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
	cfg := &config.Config{} // SimilarityThreshold not used in Put

	t.Run("Successful Cache Write", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		// mockRepo is not used by PutAnswerToCache
		service := NewAnswerCacheService(mockCache, nil, cfg)

		cacheKey := AnswerCachePrefix + baseQuizID
		expectedCachedEval := CachedAnswerEvaluation{
			Evaluation: baseEvaluation,
			Embedding:  baseUserAnswerEmbedding,
			UserAnswer: baseUserAnswerText,
		}
		expectedJSON, _ := json.Marshal(expectedCachedEval)

		mockCache.On("HSet", ctx, cacheKey, baseUserAnswerText, string(expectedJSON)).Return(nil).Once()
		mockCache.On("Expire", ctx, cacheKey, AnswerCacheExpiration).Return(nil).Once()

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.NoError(t, err)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache HSet Fails", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		service := NewAnswerCacheService(mockCache, nil, cfg)
		cacheKey := AnswerCachePrefix + baseQuizID
		expectedError := fmt.Errorf("HSet failed")

		// json.Marshal will be called, so ensure args match for HSet
		expectedCachedEval := CachedAnswerEvaluation{
			Evaluation: baseEvaluation,
			Embedding:  baseUserAnswerEmbedding,
			UserAnswer: baseUserAnswerText,
		}
		expectedJSON, _ := json.Marshal(expectedCachedEval)

		mockCache.On("HSet", ctx, cacheKey, baseUserAnswerText, string(expectedJSON)).Return(expectedError).Once()

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.ErrorIs(t, err, expectedError)
		mockCache.AssertExpectations(t)
		mockCache.AssertNotCalled(t, "Expire") // Expire should not be called if HSet fails
	})

	t.Run("Cache Expire Fails", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		service := NewAnswerCacheService(mockCache, nil, cfg)
		cacheKey := AnswerCachePrefix + baseQuizID
		expectedError := fmt.Errorf("Expire failed")

		expectedCachedEval := CachedAnswerEvaluation{
			Evaluation: baseEvaluation,
			Embedding:  baseUserAnswerEmbedding,
			UserAnswer: baseUserAnswerText,
		}
		expectedJSON, _ := json.Marshal(expectedCachedEval)

		mockCache.On("HSet", ctx, cacheKey, baseUserAnswerText, string(expectedJSON)).Return(nil).Once()
		mockCache.On("Expire", ctx, cacheKey, AnswerCacheExpiration).Return(expectedError).Once()

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.ErrorIs(t, err, expectedError)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache Service disabled (nil cache)", func(t *testing.T) {
		service := NewAnswerCacheService(nil, nil, cfg) // nil cache

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, baseEvaluation)
		assert.NoError(t, err) // Should not error, just skip
	})

	t.Run("Empty User Answer Embedding", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		service := NewAnswerCacheService(mockCache, nil, cfg)

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, []float32{}, baseEvaluation) // Empty embedding
		assert.NoError(t, err) // Skips caching
		mockCache.AssertNotCalled(t, "HSet")
		mockCache.AssertNotCalled(t, "Expire")
	})

	t.Run("Nil Evaluation", func(t *testing.T) {
		mockCache := new(MockAnswerCacheDomainCache)
		service := NewAnswerCacheService(mockCache, nil, cfg)

		err := service.PutAnswerToCache(ctx, baseQuizID, baseUserAnswerText, baseUserAnswerEmbedding, nil) // Nil evaluation
		assert.NoError(t, err) // Skips caching
		mockCache.AssertNotCalled(t, "HSet")
		mockCache.AssertNotCalled(t, "Expire")
	})
}
[end of internal/service/answer_cache_test.go]
