package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/util" // For CosineSimilarity

	"go.uber.org/zap"
)

const (
	AnswerCachePrefix     = "quizanswers:"
	AnswerCacheExpiration = 24 * time.Hour
)

// CachedAnswerEvaluation defines the structure for cached answer evaluations including embeddings
type CachedAnswerEvaluation struct {
	Evaluation *dto.CheckAnswerResponse `json:"evaluation"`
	Embedding  []float32                `json:"embedding"`
	UserAnswer string                   `json:"user_answer,omitempty"` // For debugging/logging
}

// AnswerCacheService defines the interface for answer caching operations
type AnswerCacheService interface {
	GetAnswerFromCache(ctx context.Context, quizID string, userAnswerEmbedding []float32, userAnswerText string) (*dto.CheckAnswerResponse, error)
	PutAnswerToCache(ctx context.Context, quizID string, userAnswerText string, userAnswerEmbedding []float32, evaluation *dto.CheckAnswerResponse) error
}

// answerCacheServiceImpl implements AnswerCacheService
type answerCacheServiceImpl struct {
	cache domain.Cache
	repo  domain.QuizRepository
	cfg   *config.Config
}

// NewAnswerCacheService creates a new instance of answerCacheServiceImpl
func NewAnswerCacheService(cache domain.Cache, repo domain.QuizRepository, cfg *config.Config) AnswerCacheService {
	// Handle nil cache gracefully in the service methods if it can be nil.
	// Or ensure it's never nil when NewAnswerCacheService is called.
	// For now, assume it's a valid instance.
	return &answerCacheServiceImpl{
		cache: cache,
		repo:  repo,
		cfg:   cfg,
	}
}

// GetAnswerFromCache retrieves an answer from the cache if a similar one exists.
func (s *answerCacheServiceImpl) GetAnswerFromCache(ctx context.Context, quizID string, userAnswerEmbedding []float32, userAnswerText string) (*dto.CheckAnswerResponse, error) {
	if s.cache == nil || s.cfg == nil { // Or s.cfg.Embedding.SimilarityThreshold == 0 if that's a disabled state
		logger.Get().Debug("AnswerCacheService: Cache or config not available, skipping cache lookup.", zap.String("quizID", quizID))
		return nil, nil // Not an error, just no cache service available
	}
	if len(userAnswerEmbedding) == 0 { // Should not happen if called correctly, but good check
		logger.Get().Warn("AnswerCacheService: GetAnswerFromCache called with empty userAnswerEmbedding", zap.String("quizID", quizID))
		return nil, nil
	}


	cacheKey := AnswerCachePrefix + quizID
	cachedAnswersMap, err := s.cache.HGetAll(ctx, cacheKey)
	if err != nil {
		if err == domain.ErrCacheMiss { // HGetAll might return this if key doesn't exist (depends on impl)
			logger.Get().Debug("AnswerCacheService: Cache miss (key not found)", zap.String("key", cacheKey), zap.String("quizID", quizID))
			return nil, nil
		}
		logger.Get().Error("AnswerCacheService: Cache HGetAll failed", zap.Error(err), zap.String("key", cacheKey), zap.String("quizID", quizID))
		return nil, err // Actual cache error
	}

	if len(cachedAnswersMap) == 0 {
		logger.Get().Debug("AnswerCacheService: Cache miss (empty map)", zap.String("key", cacheKey), zap.String("quizID", quizID))
		return nil, nil
	}

	for _, cachedEvalDataStr := range cachedAnswersMap {
		var cachedEntry CachedAnswerEvaluation
		if errUnmarshal := json.Unmarshal([]byte(cachedEvalDataStr), &cachedEntry); errUnmarshal != nil {
			logger.Get().Warn("AnswerCacheService: Failed to unmarshal cached answer evaluation",
				zap.Error(errUnmarshal),
				zap.String("quizID", quizID),
				zap.String("userAnswer", userAnswerText))
			continue // Skip this entry
		}

		if len(cachedEntry.Embedding) == 0 {
			logger.Get().Debug("AnswerCacheService: Skipping cached entry due to missing embedding",
				zap.String("quizID", quizID),
				zap.String("cachedUserAnswer", cachedEntry.UserAnswer),
				zap.String("userAnswer", userAnswerText))
			continue // Skip this entry
		}

		similarity, errSim := util.CosineSimilarity(userAnswerEmbedding, cachedEntry.Embedding)
		if errSim != nil {
			logger.Get().Warn("AnswerCacheService: Failed to calculate cosine similarity for cached answer",
				zap.Error(errSim),
				zap.String("quizID", quizID),
				zap.String("userAnswer", userAnswerText))
			continue // Skip this entry
		}

		if similarity >= s.cfg.Embedding.SimilarityThreshold {
			logger.Get().Info("AnswerCacheService: Cache hit - Found similar answer",
				zap.String("quizID", quizID),
				zap.String("userAnswer", userAnswerText),
				zap.Float64("similarity", similarity),
				zap.String("cachedUserAnswer", cachedEntry.UserAnswer))

			// Update model answer from latest quiz data
			if s.repo != nil { // Ensure repo is available
				quizForModelAnswer, errRepo := s.repo.GetQuizByID(quizID)
				if errRepo != nil {
					logger.Get().Error("AnswerCacheService: Failed to get quiz by ID for updating model answer in cache hit",
						zap.Error(errRepo),
						zap.String("quizID", quizID))
					// Return the cached evaluation anyway, as updating model answer is an enhancement
				} else if quizForModelAnswer != nil {
					cachedEntry.Evaluation.ModelAnswer = strings.Join(quizForModelAnswer.ModelAnswers, "\n")
				}
			}
			return cachedEntry.Evaluation, nil // Cache Hit
		}
	}

	logger.Get().Debug("AnswerCacheService: No sufficiently similar answer found in cache", zap.String("quizID", quizID), zap.String("userAnswer", userAnswerText))
	return nil, nil // Cache Miss (no similar answer found)
}

// PutAnswerToCache puts an answer evaluation into the cache.
func (s *answerCacheServiceImpl) PutAnswerToCache(ctx context.Context, quizID string, userAnswerText string, userAnswerEmbedding []float32, evaluation *dto.CheckAnswerResponse) error {
	if s.cache == nil {
		logger.Get().Debug("AnswerCacheService: Cache not available, skipping cache write.", zap.String("quizID", quizID))
		return nil // Not an error, just no cache service available
	}
	if len(userAnswerEmbedding) == 0 { // Should not happen if called correctly
		logger.Get().Warn("AnswerCacheService: PutAnswerToCache called with empty userAnswerEmbedding, not caching.", zap.String("quizID", quizID))
		return nil // Don't cache if embedding is missing
	}
	if evaluation == nil {
		logger.Get().Warn("AnswerCacheService: PutAnswerToCache called with nil evaluation, not caching.", zap.String("quizID", quizID))
		return nil
	}

	cacheKey := AnswerCachePrefix + quizID
	cachedEval := CachedAnswerEvaluation{
		Evaluation: evaluation,
		Embedding:  userAnswerEmbedding,
		UserAnswer: userAnswerText,
	}

	cachedJSON, errMarshal := json.Marshal(cachedEval)
	if errMarshal != nil {
		logger.Get().Error("AnswerCacheService: Failed to marshal answer evaluation for caching",
			zap.Error(errMarshal),
			zap.String("quizID", quizID))
		return errMarshal // Return the error
	}

	if err := s.cache.HSet(ctx, cacheKey, userAnswerText, string(cachedJSON)); err != nil {
		logger.Get().Error("AnswerCacheService: Failed to cache answer evaluation (HSet)",
			zap.Error(err),
			zap.String("quizID", quizID))
		return err
	}

	if err := s.cache.Expire(ctx, cacheKey, AnswerCacheExpiration); err != nil {
		logger.Get().Error("AnswerCacheService: Failed to set cache expiration",
			zap.Error(err),
			zap.String("quizID", quizID))
		return err // Return the error, even if HSet succeeded.
	}

	logger.Get().Info("AnswerCacheService: Answer evaluation and embedding cached successfully",
		zap.String("quizID", quizID),
		zap.String("userAnswer", userAnswerText))
	return nil
}

[end of internal/service/answer_cache.go]
