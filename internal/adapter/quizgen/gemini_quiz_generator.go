package quizgen

import (
	"bytes" // Added for gob
	"context"
	"crypto/sha256"
	"encoding/gob" // Added for gob
	"encoding/hex"
	// "encoding/json" // No longer used for cache data, only for parsing simulated LLM response string
	"encoding/json" // Keep for unmarshalling simulatedJsonResponse
	"fmt"
	"io"      // For io.EOF with gob
	"strings" // For building the prompt
	"time"    // For cache TTL

	"quiz-byte/internal/cache" // Added for caching
	"quiz-byte/internal/config" // Added for config access (e.g., TTL)
	"quiz-byte/internal/domain"
	"go.uber.org/zap"                // For logging the prompt
	"golang.org/x/sync/singleflight" // Added for singleflight
)

// hashString computes SHA256 hash of a string and returns it as a hex string.
func hashString(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// HashStringForTest is a test helper to access the unexported hashString.
func HashStringForTest(text string) string {
	return hashString(text)
}

// GeminiQuizGenerator implements the domain.QuizGenerationService interface using a conceptual LangchainGo client.
type GeminiQuizGenerator struct {
	// In a real scenario, this would be the LangchainGo Gemini client instance.
	// For example: client *gemini.GenerativeModel (or whatever the specific type is)
	langchainClient interface{}
	apiKey          string
	modelName       string
	logger          *zap.Logger
	cache           domain.Cache   // Added for caching
	config          *config.Config // Added for config access
	sfGroup         singleflight.Group // Added for singleflight
}

// NewGeminiQuizGenerator creates a new instance of GeminiQuizGenerator.
// The actual LangchainGo client initialization would happen here.
func NewGeminiQuizGenerator(apiKey string, modelName string, logger *zap.Logger, cache domain.Cache, config *config.Config) (domain.QuizGenerationService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key cannot be empty")
	}
	if modelName == "" {
		return nil, fmt.Errorf("Gemini model name cannot be empty")
	}
	if cache == nil {
		// Depending on policy, this could be a fatal error or a warning allowing operation without cache.
		// For now, let's assume it's required.
		return nil, fmt.Errorf("cache instance cannot be nil for GeminiQuizGenerator")
	}
	if config == nil {
		return nil, fmt.Errorf("config instance cannot be nil for GeminiQuizGenerator")
	}
	logger.Info("Initializing GeminiQuizGenerator", zap.String("model", modelName))
	// Placeholder for actual LangchainGo client init
	// client := langchain.NewGeminiClient(apiKey, modelName) etc.
	return &GeminiQuizGenerator{
		langchainClient: nil, // Placeholder
		apiKey:          apiKey,
		modelName:       modelName,
		logger:          logger,
		cache:           cache,  // Initialize cache
		config:          config, // Initialize config
	}, nil
}

// GenerateQuizCandidates constructs a prompt and simulates an LLM call.
// This method matches the domain.QuizGenerationService interface.
func (a *GeminiQuizGenerator) GenerateQuizCandidates(ctx context.Context, subCategoryName string, existingKeywords []string, numQuestions int) ([]*domain.NewQuizData, error) {
	generatedQuizzes := make([]*domain.NewQuizData, 0) // Initialize as empty slice

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
	promptHash := hashString(formattedPrompt)
	cacheKey := cache.GenerateCacheKey("llm_response", "gemini", promptHash)

	// Cache Check
	if a.cache != nil {
		cachedDataString, err := a.cache.Get(ctx, cacheKey)
		if err == nil { // Cache hit
			a.logger.Info("LLM response cache hit", zap.String("cacheKey", cacheKey), zap.String("promptHash", promptHash))
			var cachedQuizzesData []*domain.NewQuizData
			byteReader := bytes.NewReader([]byte(cachedDataString))
			decoder := gob.NewDecoder(byteReader)
			if errDecode := decoder.Decode(&cachedQuizzesData); errDecode == nil {
				// Basic validation for cached data
				for _, qd := range cachedQuizzesData {
					if qd.Question == "" || qd.ModelAnswer == "" || len(qd.Keywords) == 0 || qd.Difficulty == "" {
						a.logger.Warn("Cached LLM response contained incomplete quiz data (gob)", zap.Any("quiz_data", qd), zap.String("cacheKey", cacheKey))
						continue // Skip this incomplete data
					}
					generatedQuizzes = append(generatedQuizzes, qd)
				}
				// If generatedQuizzes is still empty after filtering, it means all cached items were invalid.
				// In such a case, we might want to proceed as a cache miss.
				// For now, if any valid data was found, return it.
				if len(generatedQuizzes) > 0 {
					a.logger.Info("Successfully decoded cached LLM response (gob)", zap.Int("num_quizzes_from_cache", len(generatedQuizzes)))
					return generatedQuizzes, nil
				}
				a.logger.Warn("All cached LLM quiz data items were invalid (gob)", zap.String("cacheKey", cacheKey))
				// Proceed as cache miss if all items were invalid
			} else if errDecode == io.EOF {
				a.logger.Warn("Cached LLM response data is empty (EOF) (gob)", zap.String("cacheKey", cacheKey))
			} else {
				a.logger.Error("Failed to decode cached LLM response (gob)", zap.Error(errDecode), zap.String("cacheKey", cacheKey))
			}
			// Proceed to LLM call if decoding failed
		} else if err != domain.ErrCacheMiss {
			a.logger.Error("Failed to get from cache (not a cache miss)", zap.Error(err), zap.String("cacheKey", cacheKey))
			// Proceed to LLM call, but log that cache check failed for a reason other than miss
		} else {
			a.logger.Info("LLM response cache miss", zap.String("cacheKey", cacheKey), zap.String("promptHash", promptHash))
		}
	} else {
		a.logger.Warn("Cache client is nil, skipping cache check for LLM response.")
	}

	// Cache Miss or error during cache read: Use singleflight
	res, sfErr, _ := a.sfGroup.Do(cacheKey, func() (interface{}, error) {
		a.logger.Info("Simulating LLM call (within singleflight)", zap.String("cacheKey", cacheKey), zap.String("promptHash", promptHash))

		// Simulate LLM response (hardcoded JSON string)
		simulatedResponses := []string{}
		for i := 0; i < numQuestions; i++ {
			simulatedResponses = append(simulatedResponses, fmt.Sprintf(`
			{
				"Question": "Simulated Question %d for %s (from LLM): What is the primary function of a CPU?",
				"ModelAnswer": "The primary function of a CPU is to execute instructions from computer programs.",
				"Keywords": ["cpu", "processor", "computer architecture", "question%d"],
				"Difficulty": "%s"
			}`, i+1, subCategoryName, i+1, []string{"easy", "medium", "hard"}[i%3]))
		}
		simulatedJsonResponse := fmt.Sprintf("[%s]", strings.Join(simulatedResponses, ","))
		a.logger.Info("Simulated LLM JSON response generated (singleflight)", zap.String("promptHash", promptHash))

		var llmQuizzesData []*domain.NewQuizData
		if err := json.Unmarshal([]byte(simulatedJsonResponse), &llmQuizzesData); err != nil {
			a.logger.Error("Failed to unmarshal simulated LLM JSON response (singleflight)", zap.Error(err), zap.String("json_response", simulatedJsonResponse))
			return nil, fmt.Errorf("failed to parse LLM response: %w. Response: %s", err, simulatedJsonResponse)
		}

		// Filter incomplete data from LLM before caching and returning
		validLlmQuizzesData := make([]*domain.NewQuizData, 0, len(llmQuizzesData))
		for _, qd := range llmQuizzesData {
			if qd.Question == "" || qd.ModelAnswer == "" || len(qd.Keywords) == 0 || qd.Difficulty == "" {
				a.logger.Warn("Simulated LLM generated incomplete quiz data (singleflight)", zap.Any("quiz_data", qd))
				continue
			}
			validLlmQuizzesData = append(validLlmQuizzesData, qd)
		}


		// Store in cache if successful and cache client is available
		if a.cache != nil { // Check a.cache, not s.cache
			var buffer bytes.Buffer
			encoder := gob.NewEncoder(&buffer)
			if errEncode := encoder.Encode(validLlmQuizzesData); errEncode != nil { // Cache the filtered data
				a.logger.Error("Failed to gob encode LLM response for caching (singleflight)", zap.Error(errEncode), zap.String("cacheKey", cacheKey))
				// Return data even if caching fails
				return validLlmQuizzesData, nil
			} else {
				defaultLLMResponseTTL := 24 * time.Hour
				cacheTTL := defaultLLMResponseTTL
				if a.config != nil && a.config.CacheTTLs.LLMResponse != "" {
					cacheTTL = a.config.ParseTTLStringOrDefault(a.config.CacheTTLs.LLMResponse, defaultLLMResponseTTL)
				}

				if errCacheSet := a.cache.Set(ctx, cacheKey, buffer.String(), cacheTTL); errCacheSet != nil {
					a.logger.Error("Failed to set LLM response to cache (gob, singleflight)", zap.Error(errCacheSet), zap.String("cacheKey", cacheKey))
				} else {
					a.logger.Info("LLM response cached successfully (gob, singleflight)", zap.String("cacheKey", cacheKey), zap.Duration("ttl", cacheTTL))
				}
			}
		}
		return validLlmQuizzesData, nil
	})

	if sfErr != nil {
		return nil, sfErr
	}

	if finalQuizzes, ok := res.([]*domain.NewQuizData); ok {
		if len(finalQuizzes) == 0 && numQuestions > 0 {
			a.logger.Warn("LLM simulation returned empty list (after filtering, post-singleflight) but questions were requested", zap.Int("num_requested", numQuestions))
		}
		a.logger.Info("Successfully processed LLM response (singleflight)", zap.Int("num_quizzes_generated", len(finalQuizzes)))
		return finalQuizzes, nil
	}

	return nil, fmt.Errorf("unexpected type from singleflight.Do for LLM response: %T", res)
}

// Static assertion to ensure GeminiQuizGenerator implements QuizGenerationService
var _ domain.QuizGenerationService = (*GeminiQuizGenerator)(nil)
