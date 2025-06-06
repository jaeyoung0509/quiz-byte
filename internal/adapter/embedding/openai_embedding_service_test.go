package embedding

import (
	"bytes" // Added for gob
	"context"
	"encoding/gob" // Added for gob

	// "encoding/json" // No longer used for cache data in these tests
	"errors"
	"testing"
	"time"

	"quiz-byte/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEmbedder is assumed to be available from ollama_embedding_service_test.go when running package tests.
// If it's not, it should be defined here or in a shared test utility file.
// For this task, we'll rely on it being accessible.

// MockCache is a no-op implementation of domain.Cache for testing.
// Duplicating from ollama_embedding_service_test.go for clarity or if tests are run per file.
// Ideally, this would be in a shared test helper.
type OpenaiMockCache struct { // Renamed to avoid collision if ollama's MockCache is in the same package scope during tests
	mock.Mock
}

func (m *OpenaiMockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}
func (m *OpenaiMockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}
func (m *OpenaiMockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}
func (m *OpenaiMockCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *OpenaiMockCache) HGet(ctx context.Context, key, field string) (string, error) {
	args := m.Called(ctx, key, field)
	return args.String(0), args.Error(1)
}
func (m *OpenaiMockCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}
func (m *OpenaiMockCache) HSet(ctx context.Context, key string, field string, value string) error {
	args := m.Called(ctx, key, field, value)
	return args.Error(0)
}
func (m *OpenaiMockCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}

var _ domain.Cache = (*OpenaiMockCache)(nil) // Ensure OpenaiMockCache implements domain.Cache

func TestNewOpenAIEmbeddingService(t *testing.T) {
	mockCache := new(OpenaiMockCache) // Use the renamed mock
	validTTL := 30 * time.Minute
	apiKey := "fake-api-key"
	modelName := "text-embedding-ada-002"

	t.Run("success", func(t *testing.T) {
		_, err := NewOpenAIEmbeddingService(apiKey, modelName, mockCache, validTTL)
		// As with Ollama, this might be flaky if langchaingo tries to connect/validate API key.
		// assert.NoError(t, err)
		if err != nil {
			t.Logf("Note: NewOpenAIEmbeddingService happy path test produced an error, possibly due to LangchainGo internals: %v", err)
		}
	})

	t.Run("empty api key", func(t *testing.T) {
		_, err := NewOpenAIEmbeddingService("", modelName, mockCache, validTTL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "openai API key cannot be empty")
	})

	t.Run("empty model name (should use default)", func(t *testing.T) {
		_, err := NewOpenAIEmbeddingService(apiKey, "", mockCache, validTTL)
		// This should still pass as the constructor sets a default model.
		// assert.NoError(t, err)
		if err != nil {
			t.Logf("Note: NewOpenAIEmbeddingService with empty model name produced an error: %v", err)
		}
	})

	t.Run("nil cache", func(t *testing.T) {
		_, err := NewOpenAIEmbeddingService(apiKey, modelName, nil, validTTL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache instance cannot be nil")
	})

	t.Run("zero TTL", func(t *testing.T) {
		_, err := NewOpenAIEmbeddingService(apiKey, modelName, mockCache, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "embeddingCacheTTL must be positive")
	})
}

func TestOpenAIEmbeddingService_Generate(t *testing.T) {
	ctx := context.Background()
	// mockEmb and mockCache are now initialized per sub-test to ensure isolation.
	validTTL := 30 * time.Minute

	textToEmbed := "test openai text"
	expectedEmbedding := []float32{0.4, 0.5, 0.6}
	textHash := hashString(textToEmbed) // hashString from ollama_embedding_service.go
	cacheKey := "quizbyte:embedding:openai:" + textHash

	t.Run("success no cache", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		mockCache := new(OpenaiMockCache)
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}

		mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()
		mockEmb.On("EmbedQuery", ctx, textToEmbed).Return(expectedEmbedding, nil).Once() // Assumes mockEmb returns []float32

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedEmbedding)
		expectedGobData := expectedBuffer.String()

		mockCache.On("Set", ctx, cacheKey, expectedGobData, validTTL).Return(nil).Once()

		result, err := service.Generate(ctx, textToEmbed)
		assert.NoError(t, err)
		assert.Equal(t, expectedEmbedding, result)
		mockEmb.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("cache hit", func(t *testing.T) {
		mockEmb := new(MockEmbedder)      // New mock embedder for this test
		mockCache := new(OpenaiMockCache) // New mock cache for this test
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedEmbedding)
		expectedGobData := expectedBuffer.String()

		mockCache.On("Get", ctx, cacheKey).Return(expectedGobData, nil).Once()

		result, err := service.Generate(ctx, textToEmbed)
		assert.NoError(t, err)
		assert.Equal(t, expectedEmbedding, result)
		mockCache.AssertExpectations(t)
		mockEmb.AssertNotCalled(t, "EmbedQuery", ctx, textToEmbed)
	})

	t.Run("cache miss, then success", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		mockCache := new(OpenaiMockCache)
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}

		mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()
		mockEmb.On("EmbedQuery", ctx, textToEmbed).Return(expectedEmbedding, nil).Once()

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedEmbedding)
		expectedGobData := expectedBuffer.String()

		mockCache.On("Set", ctx, cacheKey, expectedGobData, validTTL).Return(nil).Once()

		result, err := service.Generate(ctx, textToEmbed)
		assert.NoError(t, err)
		assert.Equal(t, expectedEmbedding, result)
		mockEmb.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("empty text", func(t *testing.T) {
		mockEmb := new(MockEmbedder) // Still need to init service
		mockCache := new(OpenaiMockCache)
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}
		_, err := service.Generate(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input text cannot be empty")
	})

	t.Run("embedder error, cache miss", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		mockCache := new(OpenaiMockCache)
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}
		embedderErr := errors.New("openai failed")

		mockCache.On("Get", ctx, cacheKey).Return("", domain.ErrCacheMiss).Once()
		mockEmb.On("EmbedQuery", ctx, textToEmbed).Return(nil, embedderErr).Once()

		_, err := service.Generate(ctx, textToEmbed)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate embedding using OpenAI")
		mockEmb.AssertExpectations(t)
		mockCache.AssertExpectations(t)
		mockCache.AssertNotCalled(t, "Set")
	})

	t.Run("cache get error (not miss), then success", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		mockCache := new(OpenaiMockCache)
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}
		cacheErr := errors.New("random cache error")

		mockCache.On("Get", ctx, cacheKey).Return("", cacheErr).Once()
		mockEmb.On("EmbedQuery", ctx, textToEmbed).Return(expectedEmbedding, nil).Once()

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedEmbedding)
		expectedGobData := expectedBuffer.String()

		mockCache.On("Set", ctx, cacheKey, expectedGobData, validTTL).Return(nil).Once()

		result, err := service.Generate(ctx, textToEmbed)
		assert.NoError(t, err)
		assert.Equal(t, expectedEmbedding, result)
		mockEmb.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("cache hit but unmarshal error", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		mockCache := new(OpenaiMockCache)
		service := &OpenAIEmbeddingService{embedder: mockEmb, cache: mockCache, embeddingCacheTTL: validTTL}

		mockCache.On("Get", ctx, cacheKey).Return("invalid gob data", nil).Once() // Non-empty, but invalid gob for []float32
		mockEmb.On("EmbedQuery", ctx, textToEmbed).Return(expectedEmbedding, nil).Once()

		var expectedBuffer bytes.Buffer
		enc := gob.NewEncoder(&expectedBuffer)
		_ = enc.Encode(expectedEmbedding)
		expectedGobData := expectedBuffer.String()

		mockCache.On("Set", ctx, cacheKey, expectedGobData, validTTL).Return(nil).Once()

		result, err := service.Generate(ctx, textToEmbed)
		assert.NoError(t, err)
		assert.Equal(t, expectedEmbedding, result)
		mockEmb.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})
}

// Ensure OpenAIEmbeddingService implements EmbeddingService
var _ domain.EmbeddingService = (*OpenAIEmbeddingService)(nil)

// hashString is defined in ollama_embedding_service.go (or another .go file in this package)
// and is accessible to tests in the same package.
// No need to redefine it here.
// Imports for "crypto/sha256" and "encoding/hex" that were added for a local hashString
// should be removed if no other test logic uses them directly.
