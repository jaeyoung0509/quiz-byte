package embedding

import (
	"bytes" // Added for gob
	"context"
	"crypto/sha256"
	"encoding/gob" // Added for gob
	"encoding/hex"
	// "encoding/json" // No longer used for cache data
	"fmt"
	"io" // For io.EOF with gob
	"time"

	"quiz-byte/internal/cache"
	// "quiz-byte/internal/config" // No longer needed here
	"quiz-byte/internal/domain"

	"github.com/tmc/langchaingo/embeddings"
	ollamaLLM "github.com/tmc/langchaingo/llms/ollama"
	// "go.uber.org/zap" // Logger can be added later if a logger field is introduced
	"golang.org/x/sync/singleflight" // Added for singleflight
)

// hashString computes SHA256 hash of a string and returns it as a hex string.
func hashString(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text)) // Error check omitted for brevity in example
	return hex.EncodeToString(hasher.Sum(nil))
}

// OllamaEmbeddingService implements the domain.EmbeddingService interface using Ollama.
type OllamaEmbeddingService struct {
	embedder          embeddings.Embedder
	cache             domain.Cache
	embeddingCacheTTL time.Duration      // Added
	sfGroup           singleflight.Group // Added for singleflight
	// logger   *zap.Logger // Can be added if logging is formally introduced
}

// NewOllamaEmbeddingService creates a new OllamaEmbeddingService.
func NewOllamaEmbeddingService(serverURL, modelName string, cache domain.Cache, embeddingCacheTTL time.Duration /*, logger *zap.Logger*/) (*OllamaEmbeddingService, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("ollama server URL cannot be empty")
	}
	if modelName == "" {
		return nil, fmt.Errorf("ollama model name cannot be empty")
	}
	if cache == nil {
		return nil, fmt.Errorf("cache instance cannot be nil for OllamaEmbeddingService")
	}
	if embeddingCacheTTL <= 0 {
		return nil, fmt.Errorf("embeddingCacheTTL must be positive for OllamaEmbeddingService")
	}
	// if logger == nil {
	// 	return nil, fmt.Errorf("logger instance cannot be nil")
	// }

	llm, err := ollamaLLM.New(
		ollamaLLM.WithModel(modelName),
		ollamaLLM.WithServerURL(serverURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LangchainGo Ollama LLM client for embedder: %w", err)
	}

	embedder, errEmbedder := embeddings.NewEmbedder(llm)
	if errEmbedder != nil {
		return nil, fmt.Errorf("failed to create generic embedder from Ollama LLM: %w", errEmbedder)
	}

	return &OllamaEmbeddingService{
		embedder:          embedder,
		cache:             cache,
		embeddingCacheTTL: embeddingCacheTTL,
		// logger:   logger,
	}, nil
}

// Generate creates an embedding for the given text using the Ollama embedder.
func (s *OllamaEmbeddingService) Generate(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("input text cannot be empty for embedding")
	}

	textHash := hashString(text)
	cacheKey := cache.GenerateCacheKey("embedding", "ollama", textHash)

	// Cache Check
	if s.cache != nil {
		cachedDataString, err := s.cache.Get(ctx, cacheKey)
		if err == nil { // Cache hit
			var embedding []float32
			byteReader := bytes.NewReader([]byte(cachedDataString))
			decoder := gob.NewDecoder(byteReader)
			if errDecode := decoder.Decode(&embedding); errDecode == nil {
				// s.logger.Debug("Embedding cache hit for ollama", zap.String("textHash", textHash))
				return embedding, nil
			} else if errDecode == io.EOF {
				// s.logger.Warn("Cached ollama embedding data is empty (EOF)", zap.String("cacheKey", cacheKey))
			} else {
				// s.logger.Error("Failed to decode cached ollama embedding", zap.Error(errDecode), zap.String("cacheKey", cacheKey))
			}
			// Proceed to generate if decoding failed
		} else if err != domain.ErrCacheMiss {
			// s.logger.Error("Failed to get from cache (ollama embedding)", zap.Error(err), zap.String("cacheKey", cacheKey))
			// Proceed to generate, but log that cache check failed
		} else {
			// s.logger.Debug("Embedding cache miss for ollama", zap.String("textHash", textHash))
		}
	}

	// Cache Miss or error during cache read: Use singleflight to fetch and cache.
	res, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// s.logger.Debug("Calling singleflight Do func for ollama embedding", zap.String("cacheKey", cacheKey))
		rawEmbedding, fetchErr := s.embedder.EmbedQuery(ctx, text)
		if fetchErr != nil {
			// s.logger.Error("Failed to generate embedding using Ollama (within singleflight)", zap.Error(fetchErr), zap.String("cacheKey", cacheKey))
			return nil, fmt.Errorf("failed to generate embedding using Ollama: %w", fetchErr)
		}

		// Convert []float64 to []float32 for consistency
		embeddingResult := make([]float32, len(rawEmbedding))
		for i, v := range rawEmbedding {
			embeddingResult[i] = float32(v)
		}

		if s.cache != nil {
			var buffer bytes.Buffer
			encoder := gob.NewEncoder(&buffer)
			if errEncode := encoder.Encode(embeddingResult); errEncode != nil {
				// s.logger.Error("Failed to gob encode ollama embedding for caching (singleflight)", zap.Error(errEncode), zap.String("cacheKey", cacheKey))
				// Return the embedding even if caching fails
				return embeddingResult, nil
			}

			cacheTTL := s.embeddingCacheTTL

			if errCacheSet := s.cache.Set(ctx, cacheKey, buffer.String(), cacheTTL); errCacheSet != nil {
				// s.logger.Error("Failed to set ollama embedding to cache (singleflight)", zap.Error(errCacheSet), zap.String("cacheKey", cacheKey))
			} else {
				// s.logger.Debug("Ollama embedding cached successfully (singleflight)", zap.String("cacheKey", cacheKey), zap.Duration("ttl", cacheTTL))
			}
		}
		return embeddingResult, nil
	})

	if err != nil {
		return nil, err
	}
	// Type assert the result from singleflight.Do
	if embedding, ok := res.([]float32); ok {
		return embedding, nil
	}

	return nil, fmt.Errorf("unexpected type from singleflight.Do for ollama embedding: %T", res)
}
