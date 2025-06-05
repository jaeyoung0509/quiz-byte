package quizgen_test

import (
	"context"
	"fmt"
	"testing"
	"time" // For MockCache

	"quiz-byte/internal/adapter/quizgen" // Use the actual package name
	"quiz-byte/internal/config"          // Added import
	"quiz-byte/internal/domain"          // For domain.Cache interface
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"bytes"         // Added for gob
	"encoding/gob"  // Added for gob
	"encoding/json" // For unmarshalling expectedQuizzesData from string literal
	"strings"       // Added for strings.Join
	"github.com/stretchr/testify/mock" // For testify mock
)

// MockCache now uses testify's mock.
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}
func (m *MockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
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
func (m *MockCache) HSet(ctx context.Context, key, field, value string) error {
	args := m.Called(ctx, key, field, value)
	return args.Error(0)
}
func (m *MockCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}

var _ domain.Cache = (*MockCache)(nil)

func TestNewGeminiQuizGenerator(t *testing.T) {
	logger := zap.NewNop()
	apiKey := "test-api-key"
	modelName := "test-model"
	mockCache := new(MockCache) // Now a testify mock
	validConfig := &config.Config{
		CacheTTLs: config.CacheTTLConfig{LLMResponse: "1h"}, // Provide a default test TTL
	}

	// Test successful creation
	svc, err := quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, mockCache, validConfig)
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	// Test with empty API key
	_, err = quizgen.NewGeminiQuizGenerator("", modelName, logger, mockCache, validConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key cannot be empty")

	// Test with empty model name
	_, err = quizgen.NewGeminiQuizGenerator(apiKey, "", logger, mockCache, validConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model name cannot be empty")

	// Test with nil cache - should error
	_, err = quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, nil, validConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache instance cannot be nil")

	// Test with nil config - should error
	_, err = quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, mockCache, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config instance cannot be nil")
}


func TestGeminiQuizGenerator_GenerateQuizCandidates_Caching(t *testing.T) {
	logger := zap.NewNop()
	apiKey := "test-api-key"
	modelName := "test-model"
	ctx := context.Background()
	subCategoryName := "Caching Test SubCategory"
	existingKeywords := []string{"cache_kw1", "cache_kw2"}
	numQuestions := 1 // Simplified for cache test

	// Expected data (simulated LLM response)
	simulatedResponses := []string{
		fmt.Sprintf(`
		{
			"Question": "Simulated Question 1 for %s (from LLM): What is the primary function of a CPU?",
			"ModelAnswer": "The primary function of a CPU is to execute instructions from computer programs.",
			"Keywords": ["cpu", "processor", "computer architecture", "question1"],
			"Difficulty": "easy"
		}`, subCategoryName),
	}
	expectedJsonResponse := fmt.Sprintf("[%s]", strings.Join(simulatedResponses, ","))
	var expectedQuizzesData []*domain.NewQuizData
	_ = json.Unmarshal([]byte(expectedJsonResponse), &expectedQuizzesData) // Pre-unmarshal for comparison


	// Prompt construction logic copied from SUT to generate expected hash/key
	promptTemplate := `
You are an expert quiz generator. Your task is to create %d unique and high-quality quiz questions
for the sub-category: "%s".

Avoid generating questions that are too similar to existing themes covered by these keywords: [%s].

For each question, provide the following information in JSON format:
1.  "question": The quiz question text.
2.  "model_answer": A concise and accurate model answer.
3.  "keywords": An array of 2-5 relevant keywords for this question.
4.  "difficulty": A string indicating the difficulty ("easy", "medium", or "hard").

Ensure your entire response is a single JSON array containing %d JSON objects, each representing a quiz.
Example for one quiz object:
{
  "question": "What is the capital of France?",
  "model_answer": "Paris",
  "keywords": ["france", "capital", "paris"],
  "difficulty": "easy"
}
`
	formattedPrompt := fmt.Sprintf(promptTemplate, numQuestions, subCategoryName, strings.Join(existingKeywords, ", "), numQuestions)
	// hashString is unexported, so we can't call it directly.
	// Instead, we rely on the service to generate the key and ensure cache interactions happen.
	// For asserting the key, we'd need to make hashString exportable or replicate logic.
	// For now, we'll use mock.Anything for the key if exact match is too complex without direct hashString access.
	// Or, we can grab the key from the first call if we are careful.
	// Let's assume we can predict the key for now if hashString becomes available or by other means.
	// For now, let's use mock.Anything or a placeholder if hash is tricky.
	// Re-evaluating: tests are in the same package, so unexported hashString is callable.
	promptHash := quizgen.HashStringForTest(formattedPrompt) // Accessing via test helper if unexported
	cacheKey := "quizbyte:llm_response:gemini:" + promptHash


	t.Run("Cache Miss", func(t *testing.T) {
		mockCache := new(MockCache)
		testConfig := &config.Config{
			CacheTTLs: config.CacheTTLConfig{LLMResponse: "15m"},
		}
		svc, err := quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, mockCache, testConfig)
		require.NoError(t, err)

		mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()
		// The Set call will use the actual JSON response from the (simulated) LLM
		// We need to ensure the mock LLM call (already in the SUT) produces `expectedJsonResponse`
	// which is then unmarshalled to expectedQuizzesData, which is then gob-encoded for caching.
		expectedTTL, _ := time.ParseDuration("15m")

	var expectedBuffer bytes.Buffer
	enc := gob.NewEncoder(&expectedBuffer)
	_ = enc.Encode(expectedQuizzesData) // expectedQuizzesData is []*domain.NewQuizData
	expectedGobData := expectedBuffer.String()

	mockCache.On("Set", ctx, cacheKey, expectedGobData, expectedTTL).Return(nil).Once()

		quizDataSlice, err := svc.GenerateQuizCandidates(ctx, subCategoryName, existingKeywords, numQuestions)
		assert.NoError(t, err)
		assert.Equal(t, expectedQuizzesData, quizDataSlice)
		mockCache.AssertExpectations(t)
	})

	t.Run("Cache Hit", func(t *testing.T) {
		mockCache := new(MockCache)
		testConfig := &config.Config{ // TTL doesn't matter for cache hit Get
			CacheTTLs: config.CacheTTLConfig{LLMResponse: "15m"},
		}
		svc, err := quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, mockCache, testConfig)
		require.NoError(t, err)

	var expectedBuffer bytes.Buffer
	enc := gob.NewEncoder(&expectedBuffer)
	_ = enc.Encode(expectedQuizzesData)
	expectedGobData := expectedBuffer.String()

	mockCache.On("Get", ctx, cacheKey).Return(expectedGobData, nil).Once()

		quizDataSlice, err := svc.GenerateQuizCandidates(ctx, subCategoryName, existingKeywords, numQuestions)
		assert.NoError(t, err)
		assert.Equal(t, expectedQuizzesData, quizDataSlice)
		mockCache.AssertExpectations(t)
		mockCache.AssertNotCalled(t, "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

}


// TestGeminiQuizGenerator_GenerateQuizCandidates_Success is replaced by _Caching tests
// func TestGeminiQuizGenerator_GenerateQuizCandidates_Success(t *testing.T) { ... }


// TestGenerateQuizCandidates_MalformedJSON (Conceptual as per subtask description)
// To truly test this, the JSON generation/fetching part in the SUT would need to be mockable
// or the internal hardcoded string would need to be made malformed for this specific test.
// The current implementation of GeminiQuizGenerator always produces valid JSON string.
// If the hardcoded string in GeminiQuizGenerator were to become malformed,
// the json.Unmarshal call would return an error, which is covered by standard error handling.
// This test case serves as a placeholder for how one might approach it if the SUT were different.
func TestGeminiQuizGenerator_GenerateQuizCandidates_MalformedJSON_Conceptual(t *testing.T) {
	t.Skip("Skipping MalformedJSON test as it requires SUT modification or specific mocking for current structure")
	// If we could inject the JSON response or if the SUT fetched it from an external source:
	// 1. Setup the service.
	// 2. Mock the LLM call to return a malformed JSON string.
	//    e.g., `mockLLM.On("Call", mock.Anything).Return("this is not json", nil)`
	// 3. Call GenerateQuizCandidates.
	// 4. Assert that an error is returned and it's related to JSON parsing.
	//    `assert.Error(t, err)`
	//    `assert.Contains(t, err.Error(), "failed to parse LLM response")`
}


// TestGenerateQuizCandidates_PromptConstruction (Conceptual - requires logger capture)
// The current GeminiQuizGenerator logs the prompt. To test this, we'd need a logger
// that writes to a buffer.
func TestGeminiQuizGenerator_GenerateQuizCandidates_PromptConstruction_Conceptual(t *testing.T) {
	t.Skip("Skipping PromptConstruction test as it requires logger capture setup")

	// Example of how it could be done with a logger that supports capturing output:
	// var logBuffer bytes.Buffer
	// logger := zap.New(zapcore.NewCore(
	//     zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
	//     zapcore.AddSync(&logBuffer),
	//     zap.InfoLevel,
	// ))
	//
	// apiKey := "test-api-key"
	// modelName := "test-model"
	// For this conceptual test, cache and config are not directly involved in prompt construction part.
	// svc, _ := quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, nil, nil)
	//
	// ctx := context.Background()
	// subCategoryName := "Test Prompt Construction"
	// existingKeywords := []string{"prompt_keyword1", "pk2"}
	// numQuestions := 1
	//
	// _, _ = svc.GenerateQuizCandidates(ctx, subCategoryName, existingKeywords, numQuestions)
	//
	// loggedOutput := logBuffer.String()
	// assert.Contains(t, loggedOutput, subCategoryName)
	// assert.Contains(t, loggedOutput, "prompt_keyword1")
	// assert.Contains(t, loggedOutput, "pk2")
	// assert.Contains(t, loggedOutput, fmt.Sprintf("create %d unique", numQuestions))
	// assert.Contains(t, loggedOutput, "JSON format")
	// assert.Contains(t, loggedOutput, "\"question\"")
	// assert.Contains(t, loggedOutput, "\"model_answer\"")
	// assert.Contains(t, loggedOutput, "\"keywords\"")
	// assert.Contains(t, loggedOutput, "\"difficulty\"")
}

// It's also useful to test how the service handles an empty or nil response from the LLM
// (if the LLM could theoretically return that for a valid request).
// The current simulation always returns data if numQuestions > 0.
func TestGeminiQuizGenerator_GenerateQuizCandidates_EmptyResponseFromLLM(t *testing.T) {
    logger := zap.NewNop()
    apiKey := "test-api-key"
    modelName := "test-model"
    mockCache := new(MockCache) // Use testify mock
    validConfig := &config.Config{
		CacheTTLs: config.CacheTTLConfig{LLMResponse: "1h"}, // Provide a test TTL
	}

    svc, err := quizgen.NewGeminiQuizGenerator(apiKey, modelName, logger, mockCache, validConfig)
    require.NoError(t, err)
    require.NotNil(t, svc)

    ctx := context.Background()
    subCategoryName := "Test Empty Response"
    existingKeywords := []string{}
    numQuestions := 0 // Requesting zero questions

	// Calculate expected cache key
	promptTemplate := `
You are an expert quiz generator. Your task is to create %d unique and high-quality quiz questions
for the sub-category: "%s".

Avoid generating questions that are too similar to existing themes covered by these keywords: [%s].

For each question, provide the following information in JSON format:
1.  "question": The quiz question text.
2.  "model_answer": A concise and accurate model answer.
3.  "keywords": An array of 2-5 relevant keywords for this question.
4.  "difficulty": A string indicating the difficulty ("easy", "medium", or "hard").

Ensure your entire response is a single JSON array containing %d JSON objects, each representing a quiz.
Example for one quiz object:
{
  "question": "What is the capital of France?",
  "model_answer": "Paris",
  "keywords": ["france", "capital", "paris"],
  "difficulty": "easy"
}
`
	formattedPrompt := fmt.Sprintf(promptTemplate, numQuestions, subCategoryName, strings.Join(existingKeywords, ", "), numQuestions)
	promptHash := quizgen.HashStringForTest(formattedPrompt)
	cacheKey := "quizbyte:llm_response:gemini:" + promptHash
	expectedTTL, _ := time.ParseDuration(validConfig.CacheTTLs.LLMResponse)

	mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()

	// Expect gob-encoded empty slice of []*domain.NewQuizData to be cached
	var emptyQuizzesData []*domain.NewQuizData
	var expectedEmptyBuffer bytes.Buffer
	emptyEnc := gob.NewEncoder(&expectedEmptyBuffer)
	_ = emptyEnc.Encode(emptyQuizzesData) // This will be a very small gob byte slice, not "[]"
	expectedEmptyGobData := expectedEmptyBuffer.String()

	mockCache.On("Set", ctx, cacheKey, expectedEmptyGobData, expectedTTL).Return(nil).Once()


    quizDataSlice, err := svc.GenerateQuizCandidates(ctx, subCategoryName, existingKeywords, numQuestions)

    assert.NoError(t, err)
    require.NotNil(t, quizDataSlice) // Should return an empty slice, not nil
    assert.Len(t, quizDataSlice, 0, "Should return an empty slice when 0 questions are requested")

	// The adapter's current simulation will produce an empty JSON array "[]" if numQuestions is 0.
	// This will be parsed correctly into an empty slice.
}
