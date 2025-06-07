package service

import (
	"bytes" // Added for gob
	"context"

	// "context" // Removed duplicate
	"crypto/sha256"
	"encoding/gob" // Added for gob
	"encoding/hex"

	// "encoding/json" // No longer used for cache data
	"io" // For io.EOF with gob
	"strings"
	"time"

	"quiz-byte/internal/cache"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/util" // For CosineSimilarity

	"go.uber.org/zap"
)

const (
	AnswerCachePrefix = "quizanswers:" // Cache prefix for answer evaluations
	// AnswerCacheExpiration = 24 * time.Hour // To be replaced by config
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
	cache                        domain.Cache
	repo                         domain.QuizRepository
	txManager                    domain.TransactionManager // Added for transaction support
	answerEvaluationTTL          time.Duration             // Added
	embeddingSimilarityThreshold float64                   // Added
}

// NewAnswerCacheService creates a new instance of answerCacheServiceImpl
func NewAnswerCacheService(
	cache domain.Cache,
	repo domain.QuizRepository,
	txManager domain.TransactionManager, // Added for transaction support
	answerEvaluationTTL time.Duration, // Added
	embeddingSimilarityThreshold float64, // Added
) AnswerCacheService {
	return &answerCacheServiceImpl{
		cache:                        cache,
		repo:                         repo,
		txManager:                    txManager,
		answerEvaluationTTL:          answerEvaluationTTL,
		embeddingSimilarityThreshold: embeddingSimilarityThreshold,
	}
}

// GetAnswerFromCache retrieves an answer from the cache if a similar one exists.
func (s *answerCacheServiceImpl) GetAnswerFromCache(ctx context.Context, quizID string, userAnswerEmbedding []float32, userAnswerText string) (*dto.CheckAnswerResponse, error) {
	if s.cache == nil {
		logger.Get().Debug("AnswerCacheService: Cache not available, skipping cache lookup.", zap.String("quizID", quizID))
		return nil, nil // Not an error, just no cache service available
	}
	if len(userAnswerEmbedding) == 0 {
		logger.Get().Warn("AnswerCacheService: GetAnswerFromCache called with empty userAnswerEmbedding", zap.String("quizID", quizID))
		return nil, nil
	}

	cacheKey := cache.GenerateCacheKey("answer", "evaluation_map", quizID)
	cachedAnswersMap, err := s.cache.HGetAll(ctx, cacheKey)
	if err != nil {
		if err == domain.ErrCacheMiss {
			logger.Get().Debug("AnswerCacheService: Cache miss (key not found)", zap.String("key", cacheKey), zap.String("quizID", quizID))
			return nil, nil
		}
		logger.Get().Error("AnswerCacheService: Cache HGetAll failed", zap.Error(err), zap.String("key", cacheKey), zap.String("quizID", quizID))
		return nil, err
	}

	if len(cachedAnswersMap) == 0 {
		logger.Get().Debug("AnswerCacheService: Cache miss (empty map)", zap.String("key", cacheKey), zap.String("quizID", quizID))
		return nil, nil
	}

	for fieldKey, cachedEvalDataStr := range cachedAnswersMap {
		var cachedEntry CachedAnswerEvaluation
		byteReader := bytes.NewReader([]byte(cachedEvalDataStr))
		decoder := gob.NewDecoder(byteReader)
		if errDecode := decoder.Decode(&cachedEntry); errDecode != nil {
			if errDecode == io.EOF {
				logger.Get().Warn("AnswerCacheService: Cached answer evaluation data is empty (EOF) (gob)",
					zap.String("quizID", quizID),
					zap.String("fieldKey", fieldKey))
			} else {
				logger.Get().Warn("AnswerCacheService: Failed to decode cached answer evaluation (gob)",
					zap.Error(errDecode),
					zap.String("quizID", quizID),
					zap.String("fieldKey", fieldKey))
			}
			continue
		}

		if cachedEntry.UserAnswer != "" && hashString(cachedEntry.UserAnswer) != fieldKey {
			logger.Get().Warn("AnswerCacheService: Decoded UserAnswer hash mismatch with fieldKey",
				zap.String("quizID", quizID),
				zap.String("decodedUserAnswer", cachedEntry.UserAnswer),
				zap.String("fieldKey", fieldKey))
		}

		if len(cachedEntry.Embedding) == 0 {
			logger.Get().Debug("AnswerCacheService: Skipping cached entry due to missing embedding",
				zap.String("quizID", quizID),
				zap.String("cachedUserAnswer", cachedEntry.UserAnswer),
				zap.String("userAnswer", userAnswerText))
			continue
		}

		similarity, errSim := util.CosineSimilarity(userAnswerEmbedding, cachedEntry.Embedding)
		if errSim != nil {
			logger.Get().Warn("AnswerCacheService: Failed to calculate cosine similarity for cached answer",
				zap.Error(errSim),
				zap.String("quizID", quizID),
				zap.String("userAnswer", userAnswerText))
			continue
		}

		if similarity >= s.embeddingSimilarityThreshold { // Use field value
			logger.Get().Info("AnswerCacheService: Cache hit - Found similar answer",
				zap.String("quizID", quizID),
				zap.String("userAnswer", userAnswerText),
				zap.Float64("similarity", similarity),
				zap.String("cachedUserAnswer", cachedEntry.UserAnswer))

			if s.repo != nil {
				quizForModelAnswer, errRepo := s.repo.GetQuizByID(ctx, quizID) // Added ctx
				if errRepo != nil {
					logger.Get().Error("AnswerCacheService: Failed to get quiz by ID for updating model answer in cache hit",
						zap.Error(errRepo),
						zap.String("quizID", quizID))
				} else if quizForModelAnswer != nil {
					cachedEntry.Evaluation.ModelAnswer = strings.Join(quizForModelAnswer.ModelAnswers, "\n")
				}
			}
			return cachedEntry.Evaluation, nil
		}
	}

	logger.Get().Debug("AnswerCacheService: No sufficiently similar answer found in cache", zap.String("quizID", quizID), zap.String("userAnswer", userAnswerText))
	return nil, nil
}

// PutAnswerToCache puts an answer evaluation into the cache.
func (s *answerCacheServiceImpl) PutAnswerToCache(ctx context.Context, quizID string, userAnswerText string, userAnswerEmbedding []float32, evaluation *dto.CheckAnswerResponse) error {
	if s.cache == nil {
		logger.Get().Debug("AnswerCacheService: Cache not available, skipping cache write.", zap.String("quizID", quizID))
		return nil
	}
	if len(userAnswerEmbedding) == 0 {
		logger.Get().Warn("AnswerCacheService: PutAnswerToCache called with empty userAnswerEmbedding, not caching.", zap.String("quizID", quizID))
		return nil
	}
	if evaluation == nil {
		logger.Get().Warn("AnswerCacheService: PutAnswerToCache called with nil evaluation, not caching.", zap.String("quizID", quizID))
		return nil
	}

	cacheKey := cache.GenerateCacheKey("answer", "evaluation_map", quizID)
	cachedEval := CachedAnswerEvaluation{
		Evaluation: evaluation,
		Embedding:  userAnswerEmbedding,
		UserAnswer: userAnswerText,
	}

	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if errEncode := encoder.Encode(cachedEval); errEncode != nil {
		logger.Get().Error("AnswerCacheService: Failed to gob encode answer evaluation for caching",
			zap.Error(errEncode),
			zap.String("quizID", quizID))
		return errEncode
	}

	fieldKey := hashString(userAnswerText)
	if err := s.cache.HSet(ctx, cacheKey, fieldKey, buffer.String()); err != nil {
		logger.Get().Error("AnswerCacheService: Failed to cache answer evaluation (HSet) (gob)",
			zap.Error(err),
			zap.String("quizID", quizID))
		return err
	}

	// Use the answerEvaluationTTL field from the struct
	if err := s.cache.Expire(ctx, cacheKey, s.answerEvaluationTTL); err != nil {
		logger.Get().Error("AnswerCacheService: Failed to set cache expiration",
			zap.Error(err),
			zap.String("quizID", quizID),
			zap.Duration("ttl", s.answerEvaluationTTL)) // Use field value
		return err
	}

	logger.Get().Info("AnswerCacheService: Answer evaluation and embedding cached successfully",
		zap.String("quizID", quizID),
		zap.String("userAnswer", userAnswerText))
	return nil
}

// hashString computes SHA256 hash of a string and returns it as a hex string.
func hashString(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
