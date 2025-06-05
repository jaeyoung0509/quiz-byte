package embedding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"quiz-byte/internal/domain"
)

// MockEmbedder is a mock type for the embeddings.Embedder interface
type MockEmbedder struct {
	mock.Mock
}

func (m *MockEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float32), args.Error(1)
}

func (m *MockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

func TestNewOllamaEmbeddingService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// This test is more of an integration test if we don't mock New/NewEmbedder from langchaingo.
		// For a unit test, we'd need to mock those, which is complex.
		// Given the current structure, we'll test the constructor's basic input validation.
		_, err := NewOllamaEmbeddingService("http://localhost:11434", "testmodel")
		assert.NoError(t, err) // This will try to connect unless langchaingo itself is mocked
	})

	t.Run("empty server URL", func(t *testing.T) {
		_, err := NewOllamaEmbeddingService("", "testmodel")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ollama server URL cannot be empty")
	})

	t.Run("empty model name", func(t *testing.T) {
		_, err := NewOllamaEmbeddingService("http://localhost:11434", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ollama model name cannot be empty")
	})
}

func TestOllamaEmbeddingService_Generate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		service := &OllamaEmbeddingService{embedder: mockEmb}
		expectedEmbedding := []float32{0.1, 0.2, 0.3} // Changed to float32
		expectedFloat32 := []float32{0.1, 0.2, 0.3}

		mockEmb.On("EmbedQuery", ctx, "test text").Return(expectedEmbedding, nil).Once()

		result, err := service.Generate(ctx, "test text")
		assert.NoError(t, err)
		assert.Equal(t, expectedFloat32, result)
		mockEmb.AssertExpectations(t)
	})

	t.Run("empty text", func(t *testing.T) {
		service := &OllamaEmbeddingService{embedder: new(MockEmbedder)} // Embedder won't be called
		_, err := service.Generate(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input text cannot be empty")
	})

	t.Run("embedder error", func(t *testing.T) {
		mockEmb := new(MockEmbedder)
		service := &OllamaEmbeddingService{embedder: mockEmb}
		embedderErr := errors.New("ollama failed")

		mockEmb.On("EmbedQuery", ctx, "test text").Return(nil, embedderErr).Once()

		_, err := service.Generate(ctx, "test text")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate embedding using Ollama")
		assert.True(t, errors.Is(err, embedderErr) || err.Error() == "failed to generate embedding using Ollama: ollama failed") // Check underlying error
		mockEmb.AssertExpectations(t)
	})
}

// Ensure OllamaEmbeddingService implements EmbeddingService
var _ domain.EmbeddingService = (*OllamaEmbeddingService)(nil)
