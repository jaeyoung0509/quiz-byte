package embedding

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/embeddings"
	ollamaLLM "github.com/tmc/langchaingo/llms/ollama"
	"quiz-byte/internal/domain"
)

// OllamaEmbeddingService implements the domain.EmbeddingService interface using Ollama.
type OllamaEmbeddingService struct {
	embedder embeddings.Embedder
}

// NewOllamaEmbeddingService creates a new OllamaEmbeddingService.
// It requires the Ollama server URL and model name.
func NewOllamaEmbeddingService(serverURL, modelName string) (*OllamaEmbeddingService, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("ollama server URL cannot be empty")
	}
	if modelName == "" {
		return nil, fmt.Errorf("ollama model name cannot be empty")
	}

	llm, err := ollamaLLM.New(
		ollamaLLM.WithModel(modelName),
		ollamaLLM.WithServerURL(serverURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LangchainGo Ollama LLM client for embedder: %w", err)
	}

	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return nil, fmt.Errorf("failed to create generic embedder from Ollama LLM: %w", err)
	}

	return &OllamaEmbeddingService{embedder: embedder}, nil
}

// Generate creates an embedding for the given text using the Ollama embedder.
func (s *OllamaEmbeddingService) Generate(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("input text cannot be empty for embedding")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding using Ollama: %w", err)
	}

	// Convert []float64 to []float32 for consistency
	float32Embedding := make([]float32, len(embedding))
	for i, v := range embedding {
		float32Embedding[i] = float32(v)
	}
	return float32Embedding, nil
}
