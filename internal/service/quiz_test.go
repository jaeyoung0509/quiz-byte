package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	// "time" // Removed unused import

	// Removed "encoding/json" as it's not directly used by these tests anymore for cache data setup

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"

	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/mock" // Removed unused import
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

// Mocks are now in mocks_test.go

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

// TestInvalidateQuizCache has been removed as the method is no longer part of the QuizService interface.
