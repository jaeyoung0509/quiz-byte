package service

import (
	"context"
	"fmt"
	"math"
	// "github.com/sashabaranov/go-openai" // Replaced by LangchainGo for embeddings

	"github.com/tmc/langchaingo/embeddings"
	ollamaLLM "github.com/tmc/langchaingo/llms/ollama" // Using LLM for embedder
	openaiLLM "github.com/tmc/langchaingo/llms/openai" // Using LLM for embedder

	"quiz-byte/internal/config"
)

// GenerateOpenAIEmbedding creates an embedding for the given text using OpenAI's API.
// This function is now commented out in favor of the new GenerateEmbedding function
// that uses LangchainGo and supports multiple embedding sources.
// func GenerateOpenAIEmbedding(ctx context.Context, text string, apiKey string) ([]float32, error) {
// 	if apiKey == "" {
// 		return nil, fmt.Errorf("OpenAI API key is missing")
// 	}
// 	if text == "" {
// 		return nil, fmt.Errorf("input text cannot be empty for embedding")
// 	}

// 	client := openai.NewClient(apiKey)
// 	request := openai.EmbeddingRequest{
// 		Input: []string{text},
// 		Model: openai.AdaEmbeddingV2, // Using Ada v2 model as specified
// 	}

// 	resp, err := client.CreateEmbeddings(ctx, request)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create embeddings: %w", err)
// 	}

// 	if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
// 		return nil, fmt.Errorf("received empty embedding data from OpenAI")
// 	}

// 	return resp.Data[0].Embedding, nil
// }

func GenerateEmbedding(ctx context.Context, text string, cfg *config.Config) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("input text cannot be empty for embedding")
	}

	var embedder embeddings.Embedder
	var err error

	switch cfg.Embedding.Source {
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is missing in config for openai embedding source")
		}
		// Create LLM client and then generic embedder
		// For OpenAI, text-embedding-ada-002 is a common choice.
		// Note: Ensure the model specified is compatible with embeddings.
		llm, oaiErr := openaiLLM.New(
			openaiLLM.WithToken(cfg.OpenAIAPIKey),
			openaiLLM.WithModel("text-embedding-ada-002"), // Specifying a common embedding model
		)
		if oaiErr != nil {
			return nil, fmt.Errorf("failed to create LangchainGo OpenAI LLM client for embedder: %w", oaiErr)
		}
		embedder, err = embeddings.NewEmbedder(llm)
		if err != nil {
			return nil, fmt.Errorf("failed to create generic embedder from OpenAI LLM: %w", err)
		}

	case "ollama":
		if cfg.Embedding.OllamaServerURL == "" {
			return nil, fmt.Errorf("Ollama server URL is missing in config for ollama embedding source")
		}
		if cfg.Embedding.OllamaModel == "" {
			return nil, fmt.Errorf("Ollama model name is missing in config for ollama embedding source")
		}

		llm, ollamaErr := ollamaLLM.New(
			ollamaLLM.WithModel(cfg.Embedding.OllamaModel),
			ollamaLLM.WithServerURL(cfg.Embedding.OllamaServerURL),
		)
		if ollamaErr != nil {
			return nil, fmt.Errorf("failed to create LangchainGo Ollama LLM client for embedder: %w", ollamaErr)
		}
		embedder, err = embeddings.NewEmbedder(llm)
		if err != nil {
			return nil, fmt.Errorf("failed to create generic embedder from Ollama LLM: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported embedding source: %s", cfg.Embedding.Source)
	}

	// EmbedQuery is a method on the embedder instances (oaiEmbedder, ollamaEmbedder)
	// which are of type embeddings.Embedder.
	embedding, err := embedder.EmbedQuery(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding using %s: %w", cfg.Embedding.Source, err)
	}

	// Convert []float64 to []float32 for consistency
	float32Embedding := make([]float32, len(embedding))
	for i, v := range embedding {
		float32Embedding[i] = float32(v)
	}
	return float32Embedding, nil
}

// CosineSimilarity calculates the cosine similarity between two float32 vectors.
func CosineSimilarity(vec1 []float32, vec2 []float32) (float64, error) {
	if len(vec1) == 0 || len(vec2) == 0 {
		return 0, fmt.Errorf("input vectors cannot be empty")
	}
	if len(vec1) != len(vec2) {
		return 0, fmt.Errorf("vector dimensions do not match: %d vs %d", len(vec1), len(vec2))
	}

	var dotProduct float64
	var mag1Squared float64
	var mag2Squared float64

	for i := 0; i < len(vec1); i++ {
		dotProduct += float64(vec1[i] * vec2[i])
		mag1Squared += float64(vec1[i] * vec1[i])
		mag2Squared += float64(vec2[i] * vec2[i])
	}

	mag1 := math.Sqrt(mag1Squared)
	mag2 := math.Sqrt(mag2Squared)

	if mag1 == 0 || mag2 == 0 {
		// If either vector has zero magnitude, similarity is undefined or can be considered 0.
		// Returning 0 to avoid division by zero.
		// Depending on the use case, one might want to return an error or a specific value.
		return 0, nil // Or an error: fmt.Errorf("one or both vectors have zero magnitude")
	}

	similarity := dotProduct / (mag1 * mag2)
	return similarity, nil
}
