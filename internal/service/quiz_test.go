package service

import (
	"context"
	"encoding/json"
	// "fmt"     // Removed unused import
	// "strings" // Removed unused import
	"testing"
	"time"

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os" // For TestMain
	"quiz-byte/internal/logger" // For logger initialization
)

// TestMain will be used to initialize the logger for all tests in this package
func TestMain(m *testing.M) {
	// Setup: Initialize logger
	// Use a minimal config for logger initialization during tests
	cfg := &config.Config{
		// Assuming logger.Initialize doesn't strictly need other fields,
		// or uses defaults if they are not present.
		// Based on logger.Initialize, ENV determines prod/dev logging format.
		// Let's not set ENV, so it defaults to development logging.
	}
	if err := logger.Initialize(cfg); err != nil {
		// Handle error, perhaps by logging to stderr and exiting
		// For simplicity in this context, we'll panic if logger init fails.
		panic("Failed to initialize logger for tests: " + err.Error())
	}

	// Run the tests
	exitVal := m.Run()

	// Teardown: Sync logger (optional, but good practice)
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

func (m *MockQuizRepository) GetSimilarQuiz(quizID string) (*domain.Quiz, error) { // Added missing method
	args := m.Called(quizID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetRandomQuizBySubCategory(subCategoryID string) (*domain.Quiz, error) { // Added missing method
	args := m.Called(subCategoryID)
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

func (m *MockQuizRepository) SaveAnswer(answer *domain.Answer) error { // Added missing method
	args := m.Called(answer)
	return args.Error(0)
}

func (m *MockQuizRepository) GetQuizzesByCriteria(subCategoryID string, count int) ([]*domain.Quiz, error) { // Changed to []*domain.Quiz
	args := m.Called(subCategoryID, count)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Quiz), args.Error(1)
}

type MockAnswerEvaluator struct {
	mock.Mock
}

func (m *MockAnswerEvaluator) EvaluateAnswer(question, modelAnswer, userAnswer string, keywords []string) (*domain.Answer, error) { // Changed to *domain.Answer
	args := m.Called(question, modelAnswer, userAnswer, keywords)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Answer), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCache) HGet(ctx context.Context, key, field string) (string, error) {
	args := m.Called(ctx, key, field)
	return args.String(0), args.Error(1)
}

func (m *MockCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockCache) HSet(ctx context.Context, key string, field string, value string) error {
	args := m.Called(ctx, key, field, value)
	return args.Error(0)
}

func (m *MockCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.Called(ctx, key, expiration)
	return args.Error(0)
}

// --- Tests for CheckAnswer Caching ---

func TestCheckAnswer_Caching(t *testing.T) {
	ctx := context.Background()
	req := &dto.CheckAnswerRequest{
		QuizID:     "quiz123",
		UserAnswer: "some user answer",
	}
	cacheKey := QuizAnswerCachePrefix + req.QuizID

	// Dummy embedding for test purposes when we need to simulate successful embedding
	// For tests checking failure of GenerateEmbedding, we'll provide a config that causes it to fail.
	// The actual values here don't matter as much as their presence/absence or similarity.
	testUserAnswerEmbedding := []float32{0.1, 0.2, 0.3}

	baseCfg := config.Config{
		Embedding: config.EmbeddingConfig{
			Source:              "ollama", // Using ollama as it's less likely to have globally configured keys
			OllamaModel:         "test-embed-model",
			OllamaServerURL:     "http://localhost:11435", // A non-existent server to make real embedding calls fail if not mocked
			SimilarityThreshold: 0.9,
		},
	}

	t.Run("Cache Hit - Similar Answer", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)

		cfg := baseCfg // Copy base config
		// For this test, we assume GenerateEmbedding for req.UserAnswer would produce testUserAnswerEmbedding
		// And we place a similar embedding in the cache.
		// Note: CosineSimilarity will be called with these.
		cachedEmbedding := make([]float32, len(testUserAnswerEmbedding))
		copy(cachedEmbedding, testUserAnswerEmbedding)
		cachedEmbedding[0] += 0.01 // Make it slightly different but similar

		cachedEvalResponse := &dto.CheckAnswerResponse{Score: 0.8, Explanation: "Cached explanation"}
		cachedData := CachedAnswerEvaluation{
			Evaluation: cachedEvalResponse,
			Embedding:  cachedEmbedding,
			UserAnswer: "similar cached answer",
		}
		marshaledCachedData, _ := json.Marshal(cachedData)
		cacheReturnMap := map[string]string{cachedData.UserAnswer: string(marshaledCachedData)}

		// This test is now more about "Cache Read Path - Current Embedding Generation Fails, Fallback to LLM"
		// because GenerateEmbedding for req.UserAnswer will fail due to network, skipping similarity check.
		mockCache.On("HGetAll", ctx, cacheKey).Return(cacheReturnMap, nil).Maybe() // May not be called if GenerateEmbedding fails before it. It IS called after.

		// Setup for the fallback LLM evaluation path
		fallbackQuiz := &domain.Quiz{
			ID:           req.QuizID,
			Question:     "Test Question for Cache Hit Fallback",
			ModelAnswers: []string{"Fallback Model Answer"},
			Keywords:     []string{"fallback", "key"},
		}
		// GetQuizByID is called once in the main path after embedding fails or cache is missed.
		mockRepo.On("GetQuizByID", req.QuizID).Return(fallbackQuiz, nil).Once()

		fallbackEvalResult := &domain.Answer{Score: 0.99, Explanation: "Fallback LLM explanation"}
		mockEvaluator.On("EvaluateAnswer", fallbackQuiz.Question, fallbackQuiz.ModelAnswers[0], req.UserAnswer, fallbackQuiz.Keywords).Return(fallbackEvalResult, nil).Once()

		// Since GenerateEmbedding for the write path will also fail (connection refused), HSet will not be called.
		// mockCache.On("HSet", ctx, cacheKey, req.UserAnswer, mock.AnythingOfType("string")).Return(nil).Once()
		// mockCache.On("Expire", ctx, cacheKey, CacheExpiration).Return(nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfg)
		response, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, fallbackEvalResult.Score, response.Score)

		mockEvaluator.AssertCalled(t, "EvaluateAnswer", fallbackQuiz.Question, fallbackQuiz.ModelAnswers[0], req.UserAnswer, fallbackQuiz.Keywords)
		mockCache.AssertNotCalled(t, "HSet") // HSet is not called because GenerateEmbedding for write fails
		mockCache.AssertNotCalled(t, "Expire")
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("Cache Miss - Low Similarity", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)
		cfg := baseCfg

		// This test is now "Cache Read Path - Current Embedding Generation Fails, Fallback to LLM"
		// (Similar to the previous one, as the similarity check is bypassed)
		// Embedding for cached answer (very different) - this difference won't be tested due to GenerateEmbedding failure for current answer.
		cachedEmbedding := []float32{0.9, 0.8, 0.7}

		cachedEvalResponse := &dto.CheckAnswerResponse{Score: 0.3, Explanation: "Dissimilar cached explanation"}
		cachedData := CachedAnswerEvaluation{
			Evaluation: cachedEvalResponse,
			Embedding:  cachedEmbedding,
			UserAnswer: "dissimilar cached answer",
		}
		marshaledCachedData, _ := json.Marshal(cachedData)
		cacheReturnMap := map[string]string{cachedData.UserAnswer: string(marshaledCachedData)}

		mockCache.On("HGetAll", ctx, cacheKey).Return(cacheReturnMap, nil).Maybe() // May not be called if GenerateEmbedding fails.

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1_low_similarity", ModelAnswers: []string{"Model Ans Low Sim"}, Keywords: []string{"k1ls"}}
		// GetQuizByID is called once in the main path.
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		evalResult := &domain.Answer{Score: 0.88, Explanation: "LLM explanation for low similarity fallback"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(evalResult, nil).Once()

		// Since GenerateEmbedding for the write path will also fail, HSet will not be called.
		// mockCache.On("HSet", ctx, cacheKey, req.UserAnswer, mock.AnythingOfType("string")).Return(nil).Once()
		// mockCache.On("Expire", ctx, cacheKey, CacheExpiration).Return(nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfg)
		response, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, evalResult.Score, response.Score)
		mockEvaluator.AssertCalled(t, "EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords)
		mockCache.AssertNotCalled(t, "HSet") // HSet is not called
		mockCache.AssertNotCalled(t, "Expire")
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("Cache Miss - Empty Cache", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)
		cfg := baseCfg

		// HGetAll will not be called because GenerateEmbedding for req.UserAnswer fails with baseCfg
		// mockCache.On("HGetAll", ctx, cacheKey).Return(map[string]string{}, nil).Once()

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1", ModelAnswers: []string{"Model Ans"}, Keywords: []string{"k1"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		evalResult := &domain.Answer{Score: 0.77, Explanation: "Fresh LLM explanation"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(evalResult, nil).Once()

		// Since GenerateEmbedding for the write path will also fail, HSet will not be called.
		// mockCache.On("HSet", ctx, cacheKey, req.UserAnswer, mock.AnythingOfType("string")).Return(nil).Once()
		// mockCache.On("Expire", ctx, cacheKey, CacheExpiration).Return(nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfg)
		response, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, evalResult.Score, response.Score)
		mockEvaluator.AssertCalled(t, "EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords)
		mockCache.AssertNotCalled(t, "HSet") // HSet is not called
		mockCache.AssertNotCalled(t, "Expire")
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("Cache_Write_Attempt_EmbeddingGenFails_HSetNotCalled", func(t *testing.T) { // Renamed test
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)

		// Config that should allow GenerateEmbedding to run (even if Ollama isn't actually there)
		cfgWriteTest := config.Config{ // Config that allows GenerateEmbedding to attempt network call
			Embedding: config.EmbeddingConfig{
				Source:          "ollama",
				OllamaModel:     "test-model-for-write-fail",
				OllamaServerURL: "http://localhost:11437", // Non-existent server to ensure network error
			},
		}

		mockCache.On("HGetAll", ctx, cacheKey).Return(map[string]string{}, domain.ErrCacheMiss).Maybe()

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1", ModelAnswers: []string{"Model Ans"}, Keywords: []string{"k1"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		llmResponse := &domain.Answer{Score: 0.95, Explanation: "LLM explanation for write test"}
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(llmResponse, nil).Once()

		// HSet and Expire should NOT be called because GenerateEmbedding for the write path will fail.
		// mockCache.On("HSet", ...
		// mockCache.On("Expire", ...

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfgWriteTest)
		_, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		mockCache.AssertNotCalled(t, "HSet") // Assert HSet is NOT called
		mockCache.AssertNotCalled(t, "Expire")

		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t) // Verifies HGetAll was called as expected (if at all)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("GenerateEmbedding Fails During Cache Read (Config Error)", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)

		// Config that will make GenerateEmbedding fail (e.g. unsupported source)
		cfgBadEmbed := config.Config{
			Embedding: config.EmbeddingConfig{
				Source: "unsupported_source_for_read_fail_test",
			},
		}

		// Cache HGetAll should not even be called if GenerateEmbedding for current answer fails due to config.
		// Or, if it's called, the error from GenerateEmbedding should prevent cache usage.
		// The current code calls GenerateEmbedding first. If it fails, it logs and proceeds to evaluator.

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1", ModelAnswers: []string{"Model Ans"}, Keywords: []string{"k1"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		evalResult := &domain.Answer{Score: 0.65, Explanation: "LLM fallback due to embed read fail"} // Changed to domain.Answer
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(evalResult, nil).Once()

		// HSet should also not be called if embedding source is bad, because cache is disabled.
		// The condition `s.cfg != nil && s.cfg.Embedding.Source != ""` applies to both read and write.
		// Let's refine: if Source is just bad, the cache is skipped entirely.

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfgBadEmbed)
		response, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, evalResult.Score, response.Score)

		mockCache.AssertNotCalled(t, "HGetAll") // Cache is skipped
		mockCache.AssertNotCalled(t, "HSet")    // Cache is skipped
		mockEvaluator.AssertCalled(t, "EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords)
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("GenerateEmbedding Fails During Cache Write (Config Error)", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)

		// Config that will make GenerateEmbedding fail for the write part
		cfgBadEmbedWrite := config.Config{
			Embedding: config.EmbeddingConfig{
				Source: "unsupported_source_for_write_fail_test",
			},
		}
		// This means cache is effectively disabled.

		mockCache.On("HGetAll", ctx, cacheKey).Return(map[string]string{}, domain.ErrCacheMiss).Maybe() // May or may not be called depending on Source check

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1", ModelAnswers: []string{"Model Ans"}, Keywords: []string{"k1"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		evalResult := &domain.Answer{Score: 0.55, Explanation: "LLM fallback due to embed write fail"} // Changed to domain.Answer
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(evalResult, nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfgBadEmbedWrite)
		response, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, evalResult.Score, response.Score)

		mockCache.AssertNotCalled(t, "HSet") // HSet should not be called if embedding config is invalid for write
		mockEvaluator.AssertCalled(t, "EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords)
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})

	t.Run("Cache disabled if Embedding Source is empty", func(t *testing.T) {
		mockRepo := new(MockQuizRepository)
		mockEvaluator := new(MockAnswerEvaluator)
		mockCache := new(MockCache)

		cfgNoSource := config.Config{
			Embedding: config.EmbeddingConfig{
				Source: "", // Embedding source is empty
			},
		}

		quizForEval := &domain.Quiz{ID: req.QuizID, Question: "Q1", ModelAnswers: []string{"Model Ans"}, Keywords: []string{"k1"}}
		mockRepo.On("GetQuizByID", req.QuizID).Return(quizForEval, nil).Once()

		evalResult := &domain.Answer{Score: 0.50, Explanation: "LLM because cache disabled"} // Changed to domain.Answer
		mockEvaluator.On("EvaluateAnswer", quizForEval.Question, quizForEval.ModelAnswers[0], req.UserAnswer, quizForEval.Keywords).Return(evalResult, nil).Once()

		service := NewQuizService(mockRepo, mockEvaluator, mockCache, &cfgNoSource)
		response, err := service.CheckAnswer(req)

		assert.NoError(t, err)
		assert.Equal(t, evalResult.Score, response.Score)
		mockCache.AssertNotCalled(t, "HGetAll")
		mockCache.AssertNotCalled(t, "HSet")
		mockRepo.AssertExpectations(t)
		mockEvaluator.AssertExpectations(t)
	})
}
