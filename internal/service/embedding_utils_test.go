package service

import (
	"context"
	"fmt"
	// "math" // Removing unused import
	"testing"

	"quiz-byte/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateEmbedding(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty text input", func(t *testing.T) {
		cfg := &config.Config{} // Minimal config
		_, err := GenerateEmbedding(ctx, "", cfg)
		assert.Error(t, err)
		assert.EqualError(t, err, "input text cannot be empty for embedding")
	})

	t.Run("OpenAI Source - Missing API Key", func(t *testing.T) {
		cfg := &config.Config{
			Embedding: config.EmbeddingConfig{
				Source: "openai",
			},
			OpenAIAPIKey: "", // Explicitly empty
		}
		_, err := GenerateEmbedding(ctx, "hello", cfg)
		assert.Error(t, err)
		assert.EqualError(t, err, "OpenAI API key is missing in config for openai embedding source")
	})

	t.Run("OpenAI Source - Valid Config (Expect EmbedQuery error)", func(t *testing.T) {
		cfg := &config.Config{
			OpenAIAPIKey: "dummy-test-key",
			Embedding: config.EmbeddingConfig{
				Source: "openai",
			},
		}
		// This will call the actual LangchainGo NewOpenAI and then EmbedQuery.
		// Since no real OpenAI server is hit with a valid key, an error is expected from EmbedQuery.
		// We are testing that the setup for OpenAI embedder is attempted and doesn't fail before that.
		_, err := GenerateEmbedding(ctx, "hello world", cfg)
		assert.Error(t, err) // Expecting an error from the actual embedding call
		// Check that the error is from the embedding generation, not a config validation error
		if err != nil {
			assert.Contains(t, err.Error(), "failed to generate embedding using openai")
			assert.NotContains(t, err.Error(), "OpenAI API key is missing")
		}
	})

	t.Run("Ollama Source - Missing Server URL", func(t *testing.T) {
		cfg := &config.Config{
			Embedding: config.EmbeddingConfig{
				Source:          "ollama",
				OllamaModel:     "test-model",
				OllamaServerURL: "", // Explicitly empty
			},
		}
		_, err := GenerateEmbedding(ctx, "hello", cfg)
		assert.Error(t, err)
		assert.EqualError(t, err, "Ollama server URL is missing in config for ollama embedding source")
	})

	t.Run("Ollama Source - Missing Model", func(t *testing.T) {
		cfg := &config.Config{
			Embedding: config.EmbeddingConfig{
				Source:          "ollama",
				OllamaModel:     "", // Explicitly empty
				OllamaServerURL: "http://localhost:11434",
			},
		}
		_, err := GenerateEmbedding(ctx, "hello", cfg)
		assert.Error(t, err)
		assert.EqualError(t, err, "Ollama model name is missing in config for ollama embedding source")
	})

	t.Run("Ollama Source - Valid Config (Expect EmbedQuery error)", func(t *testing.T) {
		cfg := &config.Config{
			Embedding: config.EmbeddingConfig{
				Source:          "ollama",
				OllamaModel:     "test-model",
				OllamaServerURL: "http://localhost:9999", // Non-existent server to force EmbedQuery error
			},
		}
		// This will call LangchainGo NewOllama and then EmbedQuery.
		// An error is expected from EmbedQuery due to non-existent server.
		// We test that the setup for Ollama embedder is attempted.
		_, err := GenerateEmbedding(ctx, "hello ollama", cfg)
		assert.Error(t, err) // Expecting an error from the actual embedding call
		if err != nil {
			assert.Contains(t, err.Error(), "failed to generate embedding using ollama")
			assert.NotContains(t, err.Error(), "Ollama server URL is missing")
			assert.NotContains(t, err.Error(), "Ollama model name is missing")
		}
	})

	t.Run("Unsupported Source", func(t *testing.T) {
		cfg := &config.Config{
			Embedding: config.EmbeddingConfig{
				Source: "unsupported_source",
			},
		}
		_, err := GenerateEmbedding(ctx, "hello", cfg)
		assert.Error(t, err)
		assert.EqualError(t, err, "unsupported embedding source: unsupported_source")
	})
}

func TestCosineSimilarity(t *testing.T) {
	t.Run("Identical vectors", func(t *testing.T) {
		vec1 := []float32{1, 2, 3}
		vec2 := []float32{1, 2, 3}
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		assert.InDelta(t, 1.0, similarity, 0.00001)
	})

	t.Run("Orthogonal vectors", func(t *testing.T) {
		vec1 := []float32{1, 0}
		vec2 := []float32{0, 1}
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, similarity, 0.00001)
	})

	t.Run("Opposite vectors", func(t *testing.T) {
		vec1 := []float32{1, 2, 3}
		vec2 := []float32{-1, -2, -3}
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		assert.InDelta(t, -1.0, similarity, 0.00001)
	})

	t.Run("General case vectors", func(t *testing.T) {
		vec1 := []float32{1, 2, 3, 4, 5}
		vec2 := []float32{5, 4, 3, 2, 1}
		// Dot product: 5 + 8 + 9 + 8 + 5 = 35
		// Mag vec1: sqrt(1+4+9+16+25) = sqrt(55)
		// Mag vec2: sqrt(25+16+9+4+1) = sqrt(55)
		// Similarity: 35 / (sqrt(55) * sqrt(55)) = 35 / 55 = 7 / 11
		expectedSimilarity := 35.0 / 55.0
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		assert.InDelta(t, expectedSimilarity, similarity, 0.00001)
	})

	t.Run("Mismatched dimensions", func(t *testing.T) {
		vec1 := []float32{1, 2, 3}
		vec2 := []float32{1, 2}
		_, err := CosineSimilarity(vec1, vec2)
		assert.Error(t, err)
		assert.EqualError(t, err, fmt.Sprintf("vector dimensions do not match: %d vs %d", len(vec1), len(vec2)))
	})

	t.Run("Empty vector (vec1)", func(t *testing.T) {
		vec1 := []float32{}
		vec2 := []float32{1, 2, 3}
		_, err := CosineSimilarity(vec1, vec2)
		assert.Error(t, err)
		assert.EqualError(t, err, "input vectors cannot be empty")
	})

	t.Run("Empty vector (vec2)", func(t *testing.T) {
		vec1 := []float32{1, 2, 3}
		vec2 := []float32{}
		_, err := CosineSimilarity(vec1, vec2)
		assert.Error(t, err)
		assert.EqualError(t, err, "input vectors cannot be empty")
	})

	t.Run("Both empty vectors", func(t *testing.T) {
		vec1 := []float32{}
		vec2 := []float32{}
		_, err := CosineSimilarity(vec1, vec2)
		assert.Error(t, err)
		assert.EqualError(t, err, "input vectors cannot be empty")
	})

	t.Run("Zero vector (vec1)", func(t *testing.T) {
		vec1 := []float32{0, 0, 0}
		vec2 := []float32{1, 2, 3}
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		// Similarity with a zero vector is typically 0, unless the other is also zero.
		// The implementation returns 0 to avoid division by zero if mag1 or mag2 is 0.
		assert.InDelta(t, 0.0, similarity, 0.00001)
	})

	t.Run("Zero vector (vec2)", func(t *testing.T) {
		vec1 := []float32{1, 2, 3}
		vec2 := []float32{0, 0, 0}
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, similarity, 0.00001)
	})

	t.Run("Both zero vectors", func(t *testing.T) {
		vec1 := []float32{0, 0, 0}
		vec2 := []float32{0, 0, 0}
		similarity, err := CosineSimilarity(vec1, vec2)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, similarity, 0.00001) // Or could be NaN depending on strict math, but 0 is common.
	})
}
