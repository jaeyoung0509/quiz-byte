package embedding

import (
	"bytes" // Added for gob
	"context"
	// "crypto/sha256" // No longer used here, hashString is in another file
	// "encoding/hex"   // No longer used here, hashString is in another file
	"encoding/gob" // Added for gob
	// "encoding/json" // No longer used for cache data
	"fmt"
	"io" // For io.EOF with gob
	"time"

	"quiz-byte/internal/cache"
	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"

	"github.com/tmc/langchaingo/embeddings"
	openaiLLM "github.com/tmc/langchaingo/llms/openai"
	// "go.uber.org/zap" // Logger can be added later if a logger field is introduced
	"golang.org/x/sync/singleflight" // Added for singleflight
)

// OpenAIEmbeddingService implements the domain.EmbeddingService interface using OpenAI.
// hashString is now expected to be in another file in this package (e.g., ollama_embedding_service.go or a new util.go)
type OpenAIEmbeddingService struct {
	embedder embeddings.Embedder
	cache    domain.Cache
	config   *config.Config
	sfGroup  singleflight.Group // Added for singleflight
	// logger   *zap.Logger // Can be added if logging is formally introduced
}

// NewOpenAIEmbeddingService creates a new OpenAIEmbeddingService.
func NewOpenAIEmbeddingService(apiKey, modelName string, cache domain.Cache, config *config.Config /*, logger *zap.Logger*/) (*OpenAIEmbeddingService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("openai API key cannot be empty")
	}
	if modelName == "" {
		modelName = "text-embedding-ada-002" // Default model
	}
	if cache == nil {
		return nil, fmt.Errorf("cache instance cannot be nil for OpenAIEmbeddingService")
	}
	if config == nil {
		return nil, fmt.Errorf("config instance cannot be nil for OpenAIEmbeddingService")
	}
	// if logger == nil {
	// 	return nil, fmt.Errorf("logger instance cannot be nil")
	// }

	llm, err := openaiLLM.New(
		openaiLLM.WithToken(apiKey),
		openaiLLM.WithModel(modelName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LangchainGo OpenAI LLM client for embedder: %w", err)
	}

	embedder, errEmbedder := embeddings.NewEmbedder(llm)
	if errEmbedder != nil {
		return nil, fmt.Errorf("failed to create generic embedder from OpenAI LLM: %w", errEmbedder)
	}

	return &OpenAIEmbeddingService{
		embedder: embedder,
		cache:    cache,
		config:   config,
		// logger:   logger,
	}, nil
}

// Generate creates an embedding for the given text using the OpenAI embedder.
func (s *OpenAIEmbeddingService) Generate(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("input text cannot be empty for embedding")
	}

	textHash := hashString(text)
	cacheKey := cache.GenerateCacheKey("embedding", "openai", textHash)

	// Cache Check
	if s.cache != nil {
		cachedDataString, err := s.cache.Get(ctx, cacheKey)
		if err == nil { // Cache hit
			var embedding []float32
			byteReader := bytes.NewReader([]byte(cachedDataString))
			decoder := gob.NewDecoder(byteReader)
			if errDecode := decoder.Decode(&embedding); errDecode == nil {
				// s.logger.Debug("Embedding cache hit for openai", zap.String("textHash", textHash))
				return embedding, nil
			} else if errDecode == io.EOF {
				// s.logger.Warn("Cached openai embedding data is empty (EOF)", zap.String("cacheKey", cacheKey))
			} else {
				// s.logger.Error("Failed to decode cached openai embedding", zap.Error(errDecode), zap.String("cacheKey", cacheKey))
			}
			// Proceed to generate if decoding failed
		} else if err != domain.ErrCacheMiss {
			// s.logger.Error("Failed to get from cache (openai embedding)", zap.Error(err), zap.String("cacheKey", cacheKey))
			// Proceed to generate, but log that cache check failed
		} else {
			// s.logger.Debug("Embedding cache miss for openai", zap.String("textHash", textHash))
		}
	}

	// Cache Miss or error during cache read: Use singleflight to fetch and cache.
	res, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// s.logger.Debug("Calling singleflight Do func for openai embedding", zap.String("cacheKey", cacheKey))
		rawEmbedding, fetchErr := s.embedder.EmbedQuery(ctx, text)
		if fetchErr != nil {
			// s.logger.Error("Failed to generate embedding using OpenAI (within singleflight)", zap.Error(fetchErr), zap.String("cacheKey", cacheKey))
			return nil, fmt.Errorf("failed to generate embedding using OpenAI: %w", fetchErr)
		}

		if rawEmbedding == nil {
			// s.logger.Error("Received nil embedding from OpenAI without error (singleflight)", zap.String("cacheKey", cacheKey))
			return nil, fmt.Errorf("received nil embedding from OpenAI without error")
		}
		embeddingResult := make([]float32, len(rawEmbedding))
		for i, v := range rawEmbedding { // This assumes rawEmbedding is []float64
			embeddingResult[i] = float32(v)
		}

		if s.cache != nil {
			var buffer bytes.Buffer
			encoder := gob.NewEncoder(&buffer)
			if errEncode := encoder.Encode(embeddingResult); errEncode != nil {
				// s.logger.Error("Failed to gob encode openai embedding for caching (singleflight)", zap.Error(errEncode), zap.String("cacheKey", cacheKey))
				return embeddingResult, nil // Return data even if caching fails
			}

			defaultEmbeddingTTL := 168 * time.Hour // 7 days
			cacheTTL := defaultEmbeddingTTL
			if s.config != nil && s.config.CacheTTLs.Embedding != "" {
				cacheTTL = s.config.ParseTTLStringOrDefault(s.config.CacheTTLs.Embedding, defaultEmbeddingTTL)
			}

			if errCacheSet := s.cache.Set(ctx, cacheKey, buffer.String(), cacheTTL); errCacheSet != nil {
				// s.logger.Error("Failed to set openai embedding to cache (singleflight)", zap.Error(errCacheSet), zap.String("cacheKey", cacheKey))
			} else {
				// s.logger.Debug("OpenAI embedding cached successfully (singleflight)", zap.String("cacheKey", cacheKey), zap.Duration("ttl", cacheTTL))
			}
		}
		return embeddingResult, nil
	})

	if err != nil {
		return nil, err
	}

	if embedding, ok := res.([]float32); ok {
		return embedding, nil
	}

	return nil, fmt.Errorf("unexpected type from singleflight.Do for openai embedding: %T", res)
}
