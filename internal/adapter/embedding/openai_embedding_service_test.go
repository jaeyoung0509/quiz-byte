package embedding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	// MockEmbedder is already defined in ollama_embedding_service_test.go
	// If running tests for this package, it will be available.
	// For isolated file tests, it might need to be redefined or moved to a common test helper.
	// Assuming it's available for package tests.
	"quiz-byte/internal/domain"
)

func TestNewOpenAIEmbeddingService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Similar to Ollama, this is more of an integration test without deeper langchaingo mocking.
		// Tests basic input validation.
		_, err := NewOpenAIEmbeddingService("fake-api-key", "text-embedding-ada-002")
		assert.NoError(t, err) // This will try to init unless langchaingo itself is mocked
	})

	t.Run("empty api key", func(t *testing.T) {
		_, err := NewOpenAIEmbeddingService("", "text-embedding-ada-002")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "openai API key cannot be empty")
	})

	t.Run("empty model name (should use default)", func(t *testing.T) {
		// The constructor provides a default model, so this should not error out for model name.
		_, err := NewOpenAIEmbeddingService("fake-api-key", "")
		assert.NoError(t, err) // Assumes default model is handled correctly
	})
}

func TestOpenAIEmbeddingService_Generate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockEmb := new(MockEmbedder) // Defined in ollama_embedding_service_test.go
		service := &OpenAIEmbeddingService{embedder: mockEmb}
		expectedEmbedding := []float64{0.4, 0.5, 0.6}
		expectedFloat32 := []float32{0.4, 0.5, 0.6}

		mockEmb.On("EmbedQuery", ctx, "test openai text").Return(expectedEmbedding, nil).Once()

		result, err := service.Generate(ctx, "test openai text")
		assert.NoError(t, err)
		assert.Equal(t, expectedFloat32, result)
		mockEmb.AssertExpectations(t)
	})

	t.Run("empty text", func(t *testing.T) {
		service := &OpenAIEmbeddingService{embedder: new(MockEmbedder)} // Embedder won't be called
		_, err := service.Generate(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input text cannot be empty")
	})

	t.Run("embedder error", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		service := &OpenAIEmbeddingService{embedder: mockEmb}
		embedderErr := errors.New("openai failed")

		mockEmb.On("EmbedQuery", ctx, "test openai text").Return(nil, embedderErr).Once()

		_, err := service.Generate(ctx, "test openai text")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate embedding using OpenAI")
		assert.True(t, errors.Is(err, embedderErr) || err.Error() == "failed to generate embedding using OpenAI: openai failed")
		mockEmb.AssertExpectations(t)
	})
}

// Ensure OpenAIEmbeddingService implements EmbeddingService
var _ domain.EmbeddingService = (*OpenAIEmbeddingService)(nil)
