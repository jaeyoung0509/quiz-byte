package domain

import (
	"context"
)

// EmbeddingService defines the interface for generating text embeddings.
type EmbeddingService interface {
	Generate(ctx context.Context, text string) ([]float32, error)
}
