package service

import (
	"context"
	"fmt"
	"math"

	"github.com/sashabaranov/go-openai"
)

// GenerateOpenAIEmbedding creates an embedding for the given text using OpenAI's API.
func GenerateOpenAIEmbedding(ctx context.Context, text string, apiKey string) ([]float32, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is missing")
	}
	if text == "" {
		return nil, fmt.Errorf("input text cannot be empty for embedding")
	}

	client := openai.NewClient(apiKey)
	request := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2, // Using Ada v2 model as specified
	}

	resp, err := client.CreateEmbeddings(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("received empty embedding data from OpenAI")
	}

	return resp.Data[0].Embedding, nil
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
