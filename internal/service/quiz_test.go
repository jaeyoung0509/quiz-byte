package service

import (
	"bytes" // Added for gob
	"context"
	"encoding/gob" // Added for gob
	"fmt"
	"os"
	"testing"
	"time" // Needed for TTLs

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"

	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/mock" // mock is in mocks_test.go
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

// --- Tests for GetAllSubCategories Caching ---
func TestGetAllSubCategories_Caching(t *testing.T) {
	ctx := context.Background()
	testCategoryListTTLString := "15m"
	cfg := &config.Config{
		CacheTTLs: config.CacheTTLConfig{
			CategoryList: testCategoryListTTLString,
		},
	}
	expectedCategories := []string{"cat1", "cat2", "cat3"}
	cacheKey := "quizbyte:quiz_service:category_list:all" // Manually construct as per GenerateCacheKey logic

	t.Run("Cache Miss for GetAllSubCategories", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockCache := new(MockCache) // Using the mock from mocks_test.go
		// Other mocks for NewQuizService, can be nil if not used by GetAllSubCategories
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, cfg, mockEmbSvc, mockAnswerCacheSvc)

		// Cache Miss
		mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()
		mockRepo.On("GetAllSubCategories", ctx).Return(expectedCategories, nil).Once()

		// Gob encode expected categories for cache set
		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		err := enc.Encode(expectedCategories)
		assert.NoError(t, err)
		expectedGobData := expectedBuffer.String()
		expectedTTL, _ := time.ParseDuration(testCategoryListTTLString)
		mockCache.On("Set", ctx, cacheKey, expectedGobData, expectedTTL).Return(nil).Once()

		categories, err := service.GetAllSubCategories()
		assert.NoError(t, err)
		assert.Equal(t, expectedCategories, categories)
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache Hit for GetAllSubCategories", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockCache := new(MockCache)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, cfg, mockEmbSvc, mockAnswerCacheSvc)

		// Gob encode expected categories for cache hit
		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		err := enc.Encode(expectedCategories)
		assert.NoError(t, err)
		expectedGobData := expectedBuffer.String()
		mockCache.On("Get", ctx, cacheKey).Return(expectedGobData, nil).Once()

		categories, err := service.GetAllSubCategories()
		assert.NoError(t, err)
		assert.Equal(t, expectedCategories, categories)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetAllSubCategories", ctx) // Ensure repo not called
	})
}

func TestFindMatchingScoreExplanation(t *testing.T) {
	// Logger is initialized by TestMain
	details := []domain.ScoreEvaluationDetail{
		{ScoreRange: "0.0-0.3", SampleAnswers: []string{"ans1"}, Explanation: "Explanation for 0.0-0.3"},
		{ScoreRange: "0.3-0.6", SampleAnswers: []string{"ans2"}, Explanation: "Explanation for 0.3-0.6"},
		{ScoreRange: "0.6-0.8", SampleAnswers: []string{"ans3"}, Explanation: "Explanation for 0.6-0.8"},
		{ScoreRange: "0.8-1.0", SampleAnswers: []string{"ans4"}, Explanation: "Explanation for 0.8-1.0"},
	}
	allRanges := []string{"0.0-0.3", "0.3-0.6", "0.6-0.8", "0.8-1.0"}

	tests := []struct {
		name            string
		score           float64
		details         []domain.ScoreEvaluationDetail
		allRanges       []string
		wantMatch       bool
		wantExplanation string
	}{
		{"score in first range (low)", 0.1, details, allRanges, true, "Explanation for 0.0-0.3"},
		{"score at boundary (0.3), exclusive end for first range", 0.3, details, allRanges, true, "Explanation for 0.3-0.6"}, // Matches [0.3-0.6)
		{"score in second range", 0.5, details, allRanges, true, "Explanation for 0.3-0.6"},
		{"score at boundary (0.6), exclusive end for second range", 0.6, details, allRanges, true, "Explanation for 0.6-0.8"}, // Matches [0.6-0.8)
		{"score in third range", 0.75, details, allRanges, true, "Explanation for 0.6-0.8"},
		{"score in last range (low)", 0.8, details, allRanges, true, "Explanation for 0.8-1.0"}, // Matches [0.8-1.0]
		{"score in last range (high)", 0.95, details, allRanges, true, "Explanation for 0.8-1.0"},
		{"score at max (1.0)", 1.0, details, allRanges, true, "Explanation for 0.8-1.0"},
		{"score too low", -0.1, details, allRanges, false, ""},
		{"score too high", 1.1, details, allRanges, false, ""},
		{"empty details", 0.5, []domain.ScoreEvaluationDetail{}, allRanges, false, ""},
		{"nil details", 0.5, nil, allRanges, false, ""},
		{
			"malformed range in details", 0.5,
			[]domain.ScoreEvaluationDetail{{ScoreRange: "0.x-0.5", Explanation: "bad"}},
			[]string{"0.x-0.5"},
			false, "",
		},
		{
			"explanation empty", 0.1,
			[]domain.ScoreEvaluationDetail{{ScoreRange: "0.0-0.3", Explanation: ""}},
			[]string{"0.0-0.3"},
			false, "", // Expect no match if explanation is empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			explanation, matched := findMatchingScoreExplanation(tt.score, tt.details, tt.allRanges)
			if matched != tt.wantMatch {
				t.Errorf("findMatchingScoreExplanation() matched = %v, want %v", matched, tt.wantMatch)
			}
			if explanation != tt.wantExplanation {
				t.Errorf("findMatchingScoreExplanation() explanation = %s, want %s", explanation, tt.wantExplanation)
			}
		})
	}
}

// --- Tests for GetBulkQuizzes Caching ---
func TestGetBulkQuizzes_Caching(t *testing.T) {
	ctx := context.Background()
	testQuizListTTLString := "5m"
	cfg := &config.Config{
		CacheTTLs: config.CacheTTLConfig{
			QuizList: testQuizListTTLString,
		},
	}
	subCategoryName := "Tech"
	subCategoryID := "tech-id-123"
	reqCount := 10
	cacheKey := fmt.Sprintf("quizbyte:quiz_service:quiz_list:%s:%d", subCategoryID, reqCount)

	domainQuizzes := []*domain.Quiz{
		{ID: "q1", Question: "Q1?", ModelAnswers: []string{"A1"}, Keywords: []string{"k1"}, Difficulty: domain.DifficultyEasy},
		{ID: "q2", Question: "Q2?", ModelAnswers: []string{"A2"}, Keywords: []string{"k2"}, Difficulty: domain.DifficultyMedium},
	}
	expectedResponse := &dto.BulkQuizzesResponse{
		Quizzes: []dto.QuizResponse{
			{ID: "q1", Question: "Q1?", ModelAnswers: []string{"A1"}, Keywords: []string{"k1"}, DiffLevel: "easy"},
			{ID: "q2", Question: "Q2?", ModelAnswers: []string{"A2"}, Keywords: []string{"k2"}, DiffLevel: "medium"},
		},
	}

	t.Run("Cache Miss for GetBulkQuizzes", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockCache := new(MockCache)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, cfg, mockEmbSvc, mockAnswerCacheSvc)

		mockRepo.On("GetSubCategoryIDByName", subCategoryName).Return(subCategoryID, nil).Once()
		mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()
		mockRepo.On("GetQuizzesByCriteria", subCategoryID, reqCount).Return(domainQuizzes, nil).Once()

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		err := enc.Encode(expectedResponse)
		assert.NoError(t, err)
		expectedGobData := expectedBuffer.String()
		expectedTTL, _ := time.ParseDuration(testQuizListTTLString)
		mockCache.On("Set", ctx, cacheKey, expectedGobData, expectedTTL).Return(nil).Once()

		bulkReq := &dto.BulkQuizzesRequest{SubCategory: subCategoryName, Count: reqCount}
		response, err := service.GetBulkQuizzes(bulkReq)
		assert.NoError(t, err)
		assert.Equal(t, expectedResponse, response)
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache Hit for GetBulkQuizzes", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockCache := new(MockCache)
		mockEvaluator := new(MockAnswerEvaluator)
		mockEmbSvc := new(MockEmbeddingService)
		mockAnswerCacheSvc := new(MockAnswerCacheService)

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, cfg, mockEmbSvc, mockAnswerCacheSvc)

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		err := enc.Encode(expectedResponse)
		assert.NoError(t, err)
		expectedGobData := expectedBuffer.String()

		mockRepo.On("GetSubCategoryIDByName", subCategoryName).Return(subCategoryID, nil).Once()
		mockCache.On("Get", ctx, cacheKey).Return(expectedGobData, nil).Once()

		bulkReq := &dto.BulkQuizzesRequest{SubCategory: subCategoryName, Count: reqCount}
		response, err := service.GetBulkQuizzes(bulkReq)
		assert.NoError(t, err)
		assert.Equal(t, expectedResponse, response)
		mockCache.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "GetQuizzesByCriteria", subCategoryID, reqCount)
	})
}
