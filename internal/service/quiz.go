package service

import (
	"context"
	// "encoding/json" // Removed unused import
	"quiz-byte/internal/domain"
	"quiz-byte/internal/config" // Added import
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	// "quiz-byte/internal/util" // No longer directly used for CosineSimilarity here
	"strings"
	// "time" // No longer directly used for CacheExpiration here

	"go.uber.org/zap"
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
	cache            domain.Cache // Retained for InvalidateQuizCache, though AnswerCacheService also has a cache. Consider if this is needed.
	cfg              *config.Config
	embeddingService domain.EmbeddingService
	answerCache      AnswerCacheService // New field
}

// NewQuizService creates a new instance of quizService
func NewQuizService(
	repo domain.QuizRepository,
	evaluator domain.AnswerEvaluator,
	cache domain.Cache, // Retained for now
	cfg *config.Config,
	embeddingService domain.EmbeddingService,
	answerCache AnswerCacheService, // New parameter
) QuizService {
	return &quizService{
		repo:             repo,
		evaluator:        evaluator,
		cache:            cache, // Retained for now
		cfg:              cfg,
		embeddingService: embeddingService,
		answerCache:      answerCache, // Assign new field
	}
}

// GetRandomQuiz implements QuizService
func (s *quizService) GetRandomQuiz(subCategory string) (*dto.QuizResponse, error) {
	quiz, err := s.repo.GetRandomQuiz()
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

	// 2. Original Logic: Fetch quiz, validate, call LLM (if cache miss or error)
	quiz, err := s.repo.GetQuizByID(req.QuizID)
	if err != nil {
		return nil, domain.NewInternalError("Failed to get quiz", err)
	}
	if quiz == nil {
		return nil, domain.NewQuizNotFoundError(req.QuizID)
	}

	// Create and validate answer
	answer := domain.NewAnswer(req.QuizID, req.UserAnswer)
	if err := answer.Validate(); err != nil {
		return nil, err
	}

	// Get model answers
	if len(quiz.ModelAnswers) == 0 {
		return nil, domain.NewInternalError("No model answer found", nil)
	}

	// Evaluate answer with LLM
	evaluatedAnswer, err := s.evaluator.EvaluateAnswer(
		quiz.Question,
		quiz.ModelAnswers[0], // 현재는 첫 번째 모범답안만 사용
		req.UserAnswer,
		quiz.Keywords,
	)
	if err != nil {
		return nil, domain.NewLLMServiceError(err)
	}

	// Construct full response
	response := &dto.CheckAnswerResponse{
		Score:          evaluatedAnswer.Score,
		Explanation:    evaluatedAnswer.Explanation,
		KeywordMatches: evaluatedAnswer.KeywordMatches,
		Completeness:   evaluatedAnswer.Completeness,
		Relevance:      evaluatedAnswer.Relevance,
		Accuracy:       evaluatedAnswer.Accuracy,
		ModelAnswer:    strings.Join(quiz.ModelAnswers, "\n"),
	}

	// 3. Cache Write Logic (delegated to AnswerCacheService)
	if s.answerCache != nil && errEmbed == nil && len(userAnswerEmbedding) > 0 && response != nil {
		errCachePut := s.answerCache.PutAnswerToCache(ctx, req.QuizID, req.UserAnswer, userAnswerEmbedding, response)
		if errCachePut != nil {
			// Log errors (actual errors are logged by AnswerCacheService)
			logger.Get().Error("QuizService: Error putting answer to AnswerCacheService",
				zap.Error(errCachePut),
				zap.String("quizID", req.QuizID))
			// Do not return error to client, just log it.
		}
	}

	return response, nil
}

// tryGetEvaluationFromCachedItem is removed as its logic is now in AnswerCacheService.

// GetAllSubCategories implements QuizService
func (s *quizService) GetAllSubCategories() ([]string, error) {
	categories, err := s.repo.GetAllSubCategories(context.Background()) // Added context
	if err != nil {
		return nil, domain.NewInternalError("Failed to get subcategories", err)
	}
	return categories, nil
}

// GetBulkQuizzes implements QuizService
func (s *quizService) GetBulkQuizzes(req *dto.BulkQuizzesRequest) (*dto.BulkQuizzesResponse, error) {
	// Validate Count (although handler also does, good to have service layer validation)
	if req.Count <= 0 {
		req.Count = 10 // Default to 10 if invalid count is somehow passed
	}
	if req.Count > 50 {
		req.Count = 50 // Cap at 50
	}

	// Get subcategory ID using case-insensitive comparison
	subCategoryID, err := s.repo.GetSubCategoryIDByName(req.SubCategory)
	if err != nil {
		return nil, domain.NewInternalError("Failed to get subcategory ID", err)
	}
	if subCategoryID == "" {
		return nil, domain.NewInvalidCategoryError(req.SubCategory)
	}

	domainQuizzes, err := s.repo.GetQuizzesByCriteria(subCategoryID, req.Count)
	if err != nil {
		return nil, domain.NewInternalError("Failed to get bulk quizzes from repository", err)
	}

	if len(domainQuizzes) == 0 {
		return &dto.BulkQuizzesResponse{
			Quizzes: []dto.QuizResponse{},
		}, nil
	}

	quizResponses := make([]dto.QuizResponse, 0, len(domainQuizzes))
	for _, quiz := range domainQuizzes {
		quizResponses = append(quizResponses, dto.QuizResponse{
			ID:           quiz.ID,
			Question:     quiz.Question,
			ModelAnswers: quiz.ModelAnswers,
			Keywords:     quiz.Keywords,
			DiffLevel:    quiz.DifficultyToString(),
		})
	}

	return &dto.BulkQuizzesResponse{
		Quizzes: quizResponses,
	}, nil
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

	cacheKey := AnswerCachePrefix + quizID // Using the constant from answer_cache.go
	err := s.cache.Delete(ctx, cacheKey)

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
