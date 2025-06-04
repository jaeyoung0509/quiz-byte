package embedding

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/embeddings"
	openaiLLM "github.com/tmc/langchaingo/llms/openai"
	"quiz-byte/internal/domain"
)

// OpenAIEmbeddingService implements the domain.EmbeddingService interface using OpenAI.
type OpenAIEmbeddingService struct {
	embedder embeddings.Embedder
}

// NewOpenAIEmbeddingService creates a new OpenAIEmbeddingService.
// It requires the OpenAI API key and model name.
func NewOpenAIEmbeddingService(apiKey, modelName string) (*OpenAIEmbeddingService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("openai API key cannot be empty")
	}
	if modelName == "" {
		// Default to a common embedding model if not specified, or return an error
		// For this example, let's use "text-embedding-ada-002" as a default
		// Consider making this a hard requirement by returning an error if modelName is empty
		modelName = "text-embedding-ada-002"
	}

	llm, err := openaiLLM.New(
		openaiLLM.WithToken(apiKey),
		openaiLLM.WithModel(modelName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LangchainGo OpenAI LLM client for embedder: %w", err)
	}

	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return nil, fmt.Errorf("failed to create generic embedder from OpenAI LLM: %w", err)
	}

	return &OpenAIEmbeddingService{embedder: embedder}, nil
}

// Generate creates an embedding for the given text using the OpenAI embedder.
func (s *OpenAIEmbeddingService) Generate(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("input text cannot be empty for embedding")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding using OpenAI: %w", err)
	}

	// Convert []float64 to []float32 for consistency
	float32Embedding := make([]float32, len(embedding))
	for i, v := range embedding {
		float32Embedding[i] = float32(v)
	}
	return float32Embedding, nil
}
