package service

import (
	"context"
	"quiz-byte/internal/cache" // Added import for cache key generation
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"

	// "quiz-byte/internal/util" // No longer directly used for CosineSimilarity here
	"bytes"         // Added for gob
	"crypto/sha256" // For CheckAnswer singleflight key
	"encoding/gob"  // Added for gob
	"encoding/hex"  // For CheckAnswer singleflight key
	"fmt"           // Added for singleflight error formatting
	"strings"

	// "encoding/json" // No longer needed directly for cache data
	"io"      // For io.EOF with gob
	"strconv" // Added for caching GetBulkQuizzes
	"time"    // Added for caching TTL

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight" // Added for singleflight
)

// LLMResponse represents the response from the LLM service
type LLMResponse struct {
	Score          float64  `json:"score"`           // 종합 점수
	Explanation    string   `json:"explanation"`     // 평가 설명
	KeywordMatches []string `json:"keyword_matches"` // 매칭된 키워드
	Completeness   float64  `json:"completeness"`    // 답변 완성도
	Relevance      float64  `json:"relevance"`       // 답변 관련성
	Accuracy       float64  `json:"accuracy"`        // 답변 정확도
}

// QuizService defines the interface for quiz-related operations
type QuizService interface {
	GetRandomQuiz(subCategory string) (*dto.QuizResponse, error)
	CheckAnswer(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error)
	GetAllSubCategories() ([]string, error)
	GetBulkQuizzes(req *dto.BulkQuizzesRequest) (*dto.BulkQuizzesResponse, error)
}

// quizService implements QuizService
type quizService struct {
	repo             domain.QuizRepository
	evaluator        domain.AnswerEvaluator
	cache            domain.Cache // Retained for InvalidateQuizCache
	embeddingService domain.EmbeddingService
	answerCache      AnswerCacheService
	sfGroup          singleflight.Group
	categoryListTTL  time.Duration // Added
	quizListTTL      time.Duration // Added
}

// NewQuizService creates a new instance of quizService
func NewQuizService(
	repo domain.QuizRepository,
	evaluator domain.AnswerEvaluator,
	cache domain.Cache,
	embeddingService domain.EmbeddingService,
	answerCache AnswerCacheService,
	categoryListTTL time.Duration, // Added
	quizListTTL time.Duration, // Added
) QuizService {
	return &quizService{
		repo:             repo,
		evaluator:        evaluator,
		cache:            cache,
		embeddingService: embeddingService,
		answerCache:      answerCache,
		categoryListTTL:  categoryListTTL,
		quizListTTL:      quizListTTL,
	}
}

// GetRandomQuiz implements QuizService
func (s *quizService) GetRandomQuiz(subCategory string) (*dto.QuizResponse, error) {
	ctx := context.Background() // Added context
	quiz, err := s.repo.GetRandomQuiz(ctx)
	if err != nil {
		return nil, domain.NewInternalError("Failed to get random quiz", err)
	}
	if quiz == nil {
		return nil, domain.NewInvalidCategoryError(subCategory)
	}

	return &dto.QuizResponse{
		ID:        quiz.ID,
		Question:  quiz.Question,
		Keywords:  quiz.Keywords,
		DiffLevel: quiz.DifficultyToString(),
	}, nil
}

// CheckAnswer implements QuizService
func (s *quizService) CheckAnswer(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error) {
	ctx := context.Background()
	// cacheKey variable is removed as it's now handled by AnswerCacheService

	var userAnswerEmbedding []float32
	var errEmbed error

	if s.embeddingService != nil {
		userAnswerEmbedding, errEmbed = s.embeddingService.Generate(ctx, req.UserAnswer)
		if errEmbed != nil {
			logger.Get().Warn("QuizService: Failed to generate embedding for current answer, cache will be skipped.",
				zap.Error(errEmbed),
				zap.String("quizID", req.QuizID),
				zap.String("userAnswer", req.UserAnswer))
			// errEmbed being non-nil will prevent cache usage later
		}
	} else {
		logger.Get().Debug("QuizService: Embedding service not available, cache will be skipped.", zap.String("quizID", req.QuizID))
		// userAnswerEmbedding remains nil, errEmbed remains nil
		// This state will also skip cache usage that depends on embeddings
	}

	// 1. Cache Read Logic (delegated to AnswerCacheService)
	if s.answerCache != nil && errEmbed == nil && len(userAnswerEmbedding) > 0 {
		cachedResp, errCacheGet := s.answerCache.GetAnswerFromCache(ctx, req.QuizID, userAnswerEmbedding, req.UserAnswer)
		if errCacheGet != nil {
			// Log actual errors, not misses (misses are logged by AnswerCacheService)
			logger.Get().Error("QuizService: Error getting answer from AnswerCacheService",
				zap.Error(errCacheGet),
				zap.String("quizID", req.QuizID))
			// Proceed to LLM evaluation as if it was a cache miss
		} else if cachedResp != nil {
			logger.Get().Info("QuizService: Cache hit from AnswerCacheService.", zap.String("quizID", req.QuizID))
			return cachedResp, nil // Cache Hit
		}
		// If cachedResp is nil and errCacheGet is nil, it's a cache miss, proceed to LLM.
	}

	// 2. LLM Evaluation Logic (Protected by SingleFlight)
	// Create a unique key for singleflight based on quizID and userAnswer to prevent multiple LLM calls for the same input.
	hasher := sha256.New()
	hasher.Write([]byte(req.UserAnswer))
	userAnswerHash := hex.EncodeToString(hasher.Sum(nil))
	sfKey := fmt.Sprintf("check_answer:%s:%s", req.QuizID, userAnswerHash)

	res, sfErr, _ := s.sfGroup.Do(sfKey, func() (interface{}, error) {
		logger.Get().Debug("Calling singleflight Do func for CheckAnswer", zap.String("sfKey", sfKey))

		quiz, err := s.repo.GetQuizByID(ctx, req.QuizID) // Added ctx
		if err != nil {
			return nil, domain.NewInternalError("Failed to get quiz", err)
		}
		if quiz == nil {
			return nil, domain.NewQuizNotFoundError(req.QuizID)
		}

		answer := domain.NewAnswer(req.QuizID, req.UserAnswer)
		if err := answer.Validate(); err != nil {
			return nil, err
		}

		if len(quiz.ModelAnswers) == 0 {
			return nil, domain.NewInternalError("No model answer found", nil)
		}

		evaluatedAnswer, err := s.evaluator.EvaluateAnswer(
			quiz.Question,
			quiz.ModelAnswers[0],
			req.UserAnswer,
			quiz.Keywords,
		)
		if err != nil {
			return nil, domain.NewLLMServiceError(err)
		}

		response := &dto.CheckAnswerResponse{
			Score:          evaluatedAnswer.Score,
			Explanation:    evaluatedAnswer.Explanation,
			KeywordMatches: evaluatedAnswer.KeywordMatches,
			Completeness:   evaluatedAnswer.Completeness,
			Relevance:      evaluatedAnswer.Relevance,
			Accuracy:       evaluatedAnswer.Accuracy,
			ModelAnswer:    strings.Join(quiz.ModelAnswers, "\n"),
		}

		// 3. Cache Write Logic (delegated to AnswerCacheService, happens within singleflight)
		if s.answerCache != nil && errEmbed == nil && len(userAnswerEmbedding) > 0 {
			errCachePut := s.answerCache.PutAnswerToCache(ctx, req.QuizID, req.UserAnswer, userAnswerEmbedding, response)
			if errCachePut != nil {
				logger.Get().Error("QuizService: Error putting answer to AnswerCacheService (singleflight)",
					zap.Error(errCachePut),
					zap.String("quizID", req.QuizID))
				// Log and ignore, proceed with returning the response
			}
		}
		return response, nil
	})

	if sfErr != nil {
		return nil, sfErr
	}

	if response, ok := res.(*dto.CheckAnswerResponse); ok {
		return response, nil
	}

	return nil, fmt.Errorf("unexpected type from singleflight.Do for CheckAnswer: %T", res)
}

// tryGetEvaluationFromCachedItem is removed as its logic is now in AnswerCacheService.

// GetAllSubCategories implements QuizService
func (s *quizService) GetAllSubCategories() ([]string, error) {
	ctx := context.Background() // Define context
	cacheKey := cache.GenerateCacheKey("quiz_service", "category_list", "all")

	// Cache Check
	if s.cache != nil {
		cachedDataString, err := s.cache.Get(ctx, cacheKey)
		if err == nil { // Cache hit
			var categories []string
			byteReader := bytes.NewReader([]byte(cachedDataString))
			decoder := gob.NewDecoder(byteReader)
			if errDecode := decoder.Decode(&categories); errDecode == nil {
				logger.Get().Debug("GetAllSubCategories cache hit (gob)", zap.String("cacheKey", cacheKey))
				return categories, nil
			} else if errDecode == io.EOF {
				logger.Get().Warn("GetAllSubCategories: Cached data is empty (EOF) (gob)", zap.String("cacheKey", cacheKey))
			} else {
				logger.Get().Error("GetAllSubCategories: Failed to decode cached data (gob)", zap.Error(errDecode), zap.String("cacheKey", cacheKey))
			}
			// Proceed to fetch from repo if decoding failed
		} else if err != domain.ErrCacheMiss {
			logger.Get().Error("GetAllSubCategories: Cache Get failed (not a miss)", zap.Error(err), zap.String("cacheKey", cacheKey))
			// Proceed to fetch from repo, but log that cache check failed
		} else {
			logger.Get().Debug("GetAllSubCategories cache miss", zap.String("cacheKey", cacheKey))
		}
	}

	// Cache Miss or error during cache read: Use singleflight
	res, sfErr, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		logger.Get().Debug("Calling singleflight Do func for GetAllSubCategories", zap.String("cacheKey", cacheKey))
		categories, err := s.repo.GetAllSubCategories(ctx) // ctx is already available in this scope
		if err != nil {
			return nil, domain.NewInternalError("Failed to get subcategories", err)
		}

		if s.cache != nil {
			var buffer bytes.Buffer
			encoder := gob.NewEncoder(&buffer)
			if errEncode := encoder.Encode(categories); errEncode != nil {
				logger.Get().Error("GetAllSubCategories: Failed to gob encode data for caching (singleflight)", zap.Error(errEncode), zap.String("cacheKey", cacheKey))
				// Still return categories even if caching fails, but log the error.
				return categories, nil // Return categories to prevent cache error from breaking the feature
			}

			// Use the categoryListTTL field from the struct
			if errCacheSet := s.cache.Set(ctx, cacheKey, buffer.String(), s.categoryListTTL); errCacheSet != nil {
				logger.Get().Error("GetAllSubCategories: Failed to set data to cache (gob, singleflight)", zap.Error(errCacheSet), zap.String("cacheKey", cacheKey))
			} else {
				logger.Get().Debug("GetAllSubCategories: Data cached successfully (gob, singleflight)", zap.String("cacheKey", cacheKey), zap.Duration("ttl", s.categoryListTTL))
			}
		}
		return categories, nil
	})

	if sfErr != nil {
		return nil, sfErr
	}
	if categories, ok := res.([]string); ok {
		return categories, nil
	}
	return nil, fmt.Errorf("unexpected type from singleflight.Do for GetAllSubCategories: %T", res)
}

// GetBulkQuizzes implements QuizService
func (s *quizService) GetBulkQuizzes(req *dto.BulkQuizzesRequest) (*dto.BulkQuizzesResponse, error) {
	ctx := context.Background() // Define context

	// Validate Count (although handler also does, good to have service layer validation)
	if req.Count <= 0 {
		req.Count = 10 // Default to 10 if invalid count is somehow passed
	}
	if req.Count > 50 {
		req.Count = 50 // Cap at 50
	}

	// Get subcategory ID using case-insensitive comparison
	subCategoryID, err := s.repo.GetSubCategoryIDByName(ctx, req.SubCategory) // Added ctx
	if err != nil {
		return nil, domain.NewInternalError("Failed to get subcategory ID", err)
	}
	if subCategoryID == "" {
		return nil, domain.NewInvalidCategoryError(req.SubCategory)
	}

	cacheKey := cache.GenerateCacheKey("quiz_service", "quiz_list", subCategoryID, strconv.Itoa(req.Count))

	// Cache Check
	if s.cache != nil {
		cachedDataString, errCacheGet := s.cache.Get(ctx, cacheKey)
		if errCacheGet == nil { // Cache Hit
			var response *dto.BulkQuizzesResponse
			byteReader := bytes.NewReader([]byte(cachedDataString))
			decoder := gob.NewDecoder(byteReader)
			if errDecode := decoder.Decode(&response); errDecode == nil {
				logger.Get().Debug("GetBulkQuizzes cache hit (gob)", zap.String("cacheKey", cacheKey))
				return response, nil
			} else if errDecode == io.EOF {
				logger.Get().Warn("GetBulkQuizzes: Cached data is empty (EOF) (gob)", zap.String("cacheKey", cacheKey))
			} else {
				logger.Get().Error("GetBulkQuizzes: Failed to decode cached data (gob)", zap.Error(errDecode), zap.String("cacheKey", cacheKey))
			}
			// Proceed to fetch if decode fails
		} else if errCacheGet != domain.ErrCacheMiss {
			logger.Get().Error("GetBulkQuizzes: Cache Get failed (not a miss)", zap.Error(errCacheGet), zap.String("cacheKey", cacheKey))
			// Proceed to fetch, log error
		} else {
			logger.Get().Debug("GetBulkQuizzes cache miss", zap.String("cacheKey", cacheKey))
		}
	}

	// Cache Miss or error: Use singleflight
	res, sfErr, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		logger.Get().Debug("Calling singleflight Do func for GetBulkQuizzes", zap.String("cacheKey", cacheKey))
		domainQuizzes, err := s.repo.GetQuizzesByCriteria(ctx, subCategoryID, req.Count) // Added ctx
		if err != nil {
			return nil, domain.NewInternalError("Failed to get bulk quizzes from repository", err)
		}

		quizResponses := make([]dto.QuizResponse, 0, len(domainQuizzes))
		if len(domainQuizzes) > 0 {
			for _, quiz := range domainQuizzes {
				quizResponses = append(quizResponses, dto.QuizResponse{
					ID:           quiz.ID,
					Question:     quiz.Question,
					ModelAnswers: quiz.ModelAnswers,
					Keywords:     quiz.Keywords,
					DiffLevel:    quiz.DifficultyToString(),
				})
			}
		}

		response := &dto.BulkQuizzesResponse{
			Quizzes: quizResponses,
		}

		if s.cache != nil {
			var buffer bytes.Buffer
			encoder := gob.NewEncoder(&buffer)
			if errEncode := encoder.Encode(response); errEncode != nil {
				logger.Get().Error("GetBulkQuizzes: Failed to gob encode response for caching (singleflight)", zap.Error(errEncode), zap.String("cacheKey", cacheKey))
				// Still return response even if caching fails, but log the error.
				return response, nil // Return response to prevent cache error from breaking the feature
			}

			// Use the quizListTTL field from the struct
			if errCacheSet := s.cache.Set(ctx, cacheKey, buffer.String(), s.quizListTTL); errCacheSet != nil {
				logger.Get().Error("GetBulkQuizzes: Failed to set response to cache (gob, singleflight)", zap.Error(errCacheSet), zap.String("cacheKey", cacheKey))
			} else {
				logger.Get().Debug("GetBulkQuizzes: Response cached successfully (gob, singleflight)", zap.String("cacheKey", cacheKey), zap.Duration("ttl", s.quizListTTL))
			}
		}
		return response, nil
	})

	if sfErr != nil {
		return nil, sfErr
	}
	if response, ok := res.(*dto.BulkQuizzesResponse); ok {
		return response, nil
	}
	return nil, fmt.Errorf("unexpected type from singleflight.Do for GetBulkQuizzes: %T", res)
}

// InvalidateQuizCache removes a quiz's answer evaluations from the cache.
func (s *quizService) InvalidateQuizCache(ctx context.Context, quizID string) error {
	logger.Get().Info("Attempting to invalidate cache for quizID", zap.String("quizID", quizID))

	if s.cache == nil { // Still uses the direct cache reference for this specific function
		logger.Get().Warn("QuizService: Cache client is nil, skipping cache invalidation for InvalidateQuizCache", zap.String("quizID", quizID))
		return nil
	}

	// Uses AnswerCachePrefix from the answer_cache.go (or a local constant if preferred for InvalidateQuizCache)
	// For consistency, let's assume InvalidateQuizCache should also use the prefix defined with AnswerCacheService
	// However, the current plan implies AnswerCacheService is for Get/Put, not necessarily Delete.
	// For now, keeping QuizAnswerCachePrefix if it was meant to be distinct or if this method is considered separate.
	// Let's assume it should use the new prefix for consistency.
	// If AnswerCachePrefix is not directly accessible, this method might need its own constant or take prefix from config.
	// For now, using a hardcoded prefix as before, but ideally this should be consistent.
	// Re-evaluating: InvalidateQuizCache is on quizService, it should use its own cache instance (s.cache)
	// and its own prefix if that was the design.
	// The prompt moved QuizAnswerCachePrefix to answer_cache.go as AnswerCachePrefix.
	// This means InvalidateQuizCache needs access to that, or it needs to be passed, or redefined.
	// Let's assume it's okay for InvalidateQuizCache to have its own definition or take it from somewhere.
	// For the purpose of this refactoring, let's assume QuizService should NOT use the s.cache directly for quiz answers
	// if all quiz answer caching is meant to be via AnswerCacheService.
	// This InvalidateQuizCache method seems like an outlier now.
	//
	// Option 1: AnswerCacheService gets a Delete method.
	// Option 2: InvalidateQuizCache uses the constants from AnswerCacheService.
	// Option 3: InvalidateQuizCache is removed or refactored if s.cache is only for other things.
	//
	// Given the current structure, and to minimize scope, I'll assume s.cache is still valid for this,
	// and it might need to use the `service.AnswerCachePrefix` if accessible, or this method might be
	// slated for further refactoring later if `s.cache` for quiz answers is to be fully encapsulated.
	// Let's use `service.AnswerCachePrefix` if it were exported (it is).
	// The `quiz.go` file will need to import `service` or have the const available.
	// For now, I will use the new constant name `AnswerCachePrefix` from `answer_cache.go`.
	// This implies that `quiz.go` and `answer_cache.go` are in the same package `service`.

	// Use the new cache key generation logic
	cacheKey := cache.GenerateCacheKey("answer", "evaluation_map", quizID)
	err := s.cache.Delete(ctx, cacheKey) // This will delete the entire hash map for the quizID

	if err != nil {
		logger.Get().Error("QuizService: Failed to invalidate cache",
			zap.String("quizID", quizID),
			zap.String("cacheKey", cacheKey),
			zap.Error(err))
		return domain.NewInternalError("failed to invalidate cache for quiz", err)
	}

	logger.Get().Info("QuizService: Successfully invalidated cache",
		zap.String("quizID", quizID),
		zap.String("cacheKey", cacheKey))
	return nil
}
