package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	// Removed "encoding/json" as it's not directly used by these tests anymore for cache data setup

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMain will be used to initialize the logger for all tests in this package
func TestMain(m *testing.M) {
	cfg := &config.Config{}
	if err := logger.Initialize(cfg); err != nil {
		panic("Failed to initialize logger for tests: " + err.Error())
	}
	exitVal := m.Run()
	_ = logger.Sync()
	os.Exit(exitVal)
}

// --- Mocks ---

type MockQuizRepository struct {
	mock.Mock
}

func (m *MockQuizRepository) GetRandomQuiz() (*domain.Quiz, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetQuizByID(id string) (*domain.Quiz, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetAllSubCategories() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockQuizRepository) GetSubCategoryIDByName(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockQuizRepository) GetQuizzesByCriteria(subCategoryID string, count int) ([]*domain.Quiz, error) {
	args := m.Called(subCategoryID, count)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Quiz), args.Error(1)
}

type MockAnswerEvaluator struct {
	mock.Mock
}

func (m *MockAnswerEvaluator) EvaluateAnswer(question, modelAnswer, userAnswer string, keywords []string) (*domain.Answer, error) {
	args := m.Called(question, modelAnswer, userAnswer, keywords)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Answer), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

// Methods for MockCache remain, as it's still used by InvalidateQuizCache
func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}
// Add other MockCache methods if InvalidateQuizCache or other QuizService methods use them.
// For now, only Delete is explicitly shown as used by InvalidateQuizCache.
// HGetAll, HSet, Expire are not directly used by QuizService for answer caching anymore.

type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) Generate(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

// MockAnswerCacheService
type MockAnswerCacheService struct {
	mock.Mock
}

func (m *MockAnswerCacheService) GetAnswerFromCache(ctx context.Context, quizID string, userAnswerEmbedding []float32, userAnswerText string) (*dto.CheckAnswerResponse, error) {
	args := m.Called(ctx, quizID, userAnswerEmbedding, userAnswerText)
	if args.Get(0) == nil { // If the first argument (response) is nil
		return nil, args.Error(1) // Return nil response and the error
	}
	return args.Get(0).(*dto.CheckAnswerResponse), args.Error(1)
}

func (m *MockAnswerCacheService) PutAnswerToCache(ctx context.Context, quizID string, userAnswerText string, userAnswerEmbedding []float32, evaluation *dto.CheckAnswerResponse) error {
	args := m.Called(ctx, quizID, userAnswerText, userAnswerEmbedding, evaluation)
	return args.Error(0)
}

// --- Tests for CheckAnswer Caching ---

func TestCheckAnswer_With_AnswerCacheService(t *testing.T) { // Renamed test function
	ctx := context.Background()
	baseReq := &dto.CheckAnswerRequest{
		QuizID:     "quiz123",
		UserAnswer: "some user answer",
	}

	baseCfg := config.Config{
		Embedding: config.EmbeddingConfig{
			SimilarityThreshold: 0.9, // Still used by AnswerCacheService, but QuizService is unaware of it directly
		},
	}

	t.Run("Cache Hit from AnswerCacheService (Embedding Success)", func(t *testing.T) {
		req := *baseReq

		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)
		// mockCache is still needed for NewQuizService, but not directly for these cache operations
		mockDirectCache := new(MockCache)
		cfg := baseCfg

		userAnswerEmbedding := []float32{0.1, 0.2, 0.3}
		mockEmbSvc.On("Generate", ctx, req.UserAnswer).Return(userAnswerEmbedding, nil).Once()

		expectedCachedResponse := &dto.CheckAnswerResponse{Score: 0.8, Explanation: "From AnswerCacheService"}
		mockAnswerCacheSvc.On("GetAnswerFromCache", ctx, req.QuizID, userAnswerEmbedding, req.UserAnswer).Return(expectedCachedResponse, nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockDirectCache, &cfg, mockEmbSvc, mockAnswerCacheSvc)
		response, err := service.CheckAnswer(&req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, expectedCachedResponse.Score, response.Score)
		assert.Equal(t, expectedCachedResponse.Explanation, response.Explanation)

		mockEmbSvc.AssertExpectations(t)
		mockAnswerCacheSvc.AssertExpectations(t)
		mockEvaluator.AssertNotCalled(t, "EvaluateAnswer")
		mockRepo.AssertNotCalled(t, "GetQuizByID") // Not called by QuizService directly in cache hit
	})

	t.Run("Cache Miss from AnswerCacheService (Embedding Success), LLM Fallback, Cache Write", func(t *testing.T) {
		req := *baseReq

		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)
		mockDirectCache := new(MockCache)
		cfg := baseCfg

		userAnswerEmbedding := []float32{0.1, 0.2, 0.3}
		mockEmbSvc.On("Generate", ctx, req.UserAnswer).Return(userAnswerEmbedding, nil).Once()

		// Simulate cache miss from AnswerCacheService
		mockAnswerCacheSvc.On("GetAnswerFromCache", ctx, req.QuizID, userAnswerEmbedding, req.UserAnswer).Return(nil, nil).Once()

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1", ModelAnswers: []string{"Model Ans"}, Keywords: []string{"k1"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		llmEvalResult := &domain.Answer{Score: 0.77, Explanation: "Fresh LLM explanation"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(llmEvalResult, nil).Once()

		// Construct the expected response for PutAnswerToCache
		expectedResponseToCache := &dto.CheckAnswerResponse{
			Score:          llmEvalResult.Score,
			Explanation:    llmEvalResult.Explanation,
			KeywordMatches: llmEvalResult.KeywordMatches,
			Completeness:   llmEvalResult.Completeness,
			Relevance:      llmEvalResult.Relevance,
			Accuracy:       llmEvalResult.Accuracy,
			ModelAnswer:    quizForEval.ModelAnswers[0],
		}
		mockAnswerCacheSvc.On("PutAnswerToCache", ctx, req.QuizID, req.UserAnswer, userAnswerEmbedding, expectedResponseToCache).Return(nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockDirectCache, &cfg, mockEmbSvc, mockAnswerCacheSvc)
		response, err := service.CheckAnswer(&req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, llmEvalResult.Score, response.Score)

		mockEmbSvc.AssertExpectations(t)
		mockAnswerCacheSvc.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("Embedding Generation Fails, No Cache Interaction, LLM Fallback", func(t *testing.T) {
		req := *baseReq

		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)
		mockDirectCache := new(MockCache)
		cfg := baseCfg

		mockEmbSvc.On("Generate", ctx, req.UserAnswer).Return(nil, fmt.Errorf("embedding generation failed")).Once()

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q_embed_fail", ModelAnswers: []string{"Model_embed_fail"}, Keywords: []string{"k_ef"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		llmEvalResult := &domain.Answer{Score: 0.65, Explanation: "LLM fallback due to embedding fail"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(llmEvalResult, nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockDirectCache, &cfg, mockEmbSvc, mockAnswerCacheSvc)
		response, err := service.CheckAnswer(&req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, llmEvalResult.Score, response.Score)

		mockEmbSvc.AssertExpectations(t)
		mockAnswerCacheSvc.AssertNotCalled(t, "GetAnswerFromCache")
		mockAnswerCacheSvc.AssertNotCalled(t, "PutAnswerToCache")
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("AnswerCacheService is nil, Embedding Success, LLM Fallback, No Cache Write", func(t *testing.T) {
		req := *baseReq

		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		// mockAnswerCacheSvc is not created, nil is passed
		mockDirectCache := new(MockCache)
		cfg := baseCfg

		userAnswerEmbedding := []float32{0.1, 0.2, 0.3}
		mockEmbSvc.On("Generate", ctx, req.UserAnswer).Return(userAnswerEmbedding, nil).Once()

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q_nil_ans_cache", ModelAnswers: []string{"Model_nil_ans_cache"}, Keywords: []string{"k_nac"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		llmEvalResult := &domain.Answer{Score: 0.60, Explanation: "LLM fallback, nil AnswerCacheService"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(llmEvalResult, nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockDirectCache, &cfg, mockEmbSvc, nil) // Pass nil for AnswerCacheService
		response, err := service.CheckAnswer(&req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, llmEvalResult.Score, response.Score)

		mockEmbSvc.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
		// No calls to mockAnswerCacheSvc to assert as it's nil
	})

	t.Run("EmbeddingService is nil, No Cache Interaction, LLM Fallback", func(t *testing.T) {
		req := *baseReq
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockAnswerCacheSvc := new(MockAnswerCacheService) // mock AnswerCacheService
		mockDirectCache := new(MockCache)
		cfg := baseCfg

		// EmbeddingService is nil
		service := NewQuizService(mockRepo, mockEvaluator, mockDirectCache, &cfg, nil, mockAnswerCacheSvc)

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q_nil_embed_svc", ModelAnswers: []string{"Model_nil_embed_svc"}, Keywords: []string{"k_nes"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		llmEvalResult := &domain.Answer{Score: 0.55, Explanation: "LLM fallback, nil EmbeddingService"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(llmEvalResult, nil).Once()

		response, err := service.CheckAnswer(&req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, llmEvalResult.Score, response.Score)

		mockAnswerCacheSvc.AssertNotCalled(t, "GetAnswerFromCache")
		mockAnswerCacheSvc.AssertNotCalled(t, "PutAnswerToCache")
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

}

// TestInvalidateQuizCache remains largely the same
// It uses the 'service.AnswerCachePrefix' constant now, which should be available if in same package.
// If constants are not directly accessible due to helper files or structure,
// this might need adjustment, but typically service constants are accessible within the service package.
func TestInvalidateQuizCache(t *testing.T) {
	ctx := context.Background()
	quizID := "testQuiz123"
	// Assuming AnswerCachePrefix is accessible within the service package.
	// If internal/service/answer_cache.go defines `const AnswerCachePrefix = "quizanswers:"`
	// and both quiz.go and quiz_test.go are in package `service`, this is fine.
	cacheKey := AnswerCachePrefix + quizID
	cfg := &config.Config{} // Minimal config

	t.Run("Successful Invalidation", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache) // This is the direct cache used by InvalidateQuizCache
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService) // Passed to NewQuizService

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, cfg, mockEmbSvc, mockAnswerCacheSvc)

		mockCache.On("Delete", ctx, cacheKey).Return(nil).Once()

		err := service.InvalidateQuizCache(ctx, quizID)

		assert.NoError(t, err)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizByID")
		mockEvaluator.AssertNotCalled(t, "EvaluateAnswer")
		mockEmbSvc.AssertNotCalled(t, "Generate")
		mockAnswerCacheSvc.AssertNotCalled(t, "GetAnswerFromCache")
	})

	t.Run("Cache Deletion Fails", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, cfg, mockEmbSvc, mockAnswerCacheSvc)

		expectedErr := fmt.Errorf("cache delete error")
		mockCache.On("Delete", ctx, cacheKey).Return(expectedErr).Once()

		err := service.InvalidateQuizCache(ctx, quizID)

		assert.Error(t, err)
		if internalErr, ok := err.(*domain.InternalError); ok {
			assert.Contains(t, internalErr.Message, "failed to invalidate cache for quiz")
			assert.ErrorIs(t, internalErr.Err, expectedErr)
		} else {
			t.Errorf("Expected error of type *domain.InternalError, got %T", err)
		}
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache Not Configured (nil cache client for InvalidateQuizCache)", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)

		// Pass nil for the direct cache instance used by InvalidateQuizCache
		service := NewQuizService(mockRepo, mockEvaluator, nil, cfg, mockEmbSvc, mockAnswerCacheSvc)

		err := service.InvalidateQuizCache(ctx, quizID)

		assert.NoError(t, err, "Expected no error when cache is not configured for InvalidateQuizCache")
	})
}
[end of internal/service/quiz_test.go]
