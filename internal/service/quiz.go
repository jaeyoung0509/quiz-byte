package service

import (
	"context"
	"encoding/json"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/config" // Added import
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	QuizAnswerCachePrefix = "quizanswers:"
	// SimilarityThreshold   = 0.95 // Now comes from config
	CacheExpiration     = 24 * time.Hour
	// EmbeddingVectorSize   = 1536 // Not directly used, embedding managed by GenerateEmbedding
)

// CachedAnswerEvaluation defines the structure for cached answer evaluations including embeddings
type CachedAnswerEvaluation struct {
	Evaluation *dto.CheckAnswerResponse `json:"evaluation"`
	Embedding  []float32                `json:"embedding"`
	UserAnswer string                   `json:"user_answer,omitempty"` // For debugging/logging
}

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
	repo      domain.QuizRepository
	evaluator domain.AnswerEvaluator
	cache     domain.Cache
	cfg       *config.Config // Changed from openAIAPIKey
}

// NewQuizService creates a new instance of quizService
func NewQuizService(
	repo domain.QuizRepository,
	evaluator domain.AnswerEvaluator,
	cache domain.Cache,
	cfg *config.Config, // Changed parameter
) QuizService {
	return &quizService{
		repo:      repo,
		evaluator: evaluator,
		cache:     cache,
		cfg:       cfg, // Updated field
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
	cacheKey := QuizAnswerCachePrefix + req.QuizID

	// 1. Cache Read Logic
	if s.cache != nil && s.cfg != nil && s.cfg.Embedding.Source != "" {
		currentAnswerEmbedding, errEmbed := GenerateEmbedding(ctx, req.UserAnswer, s.cfg)
		if errEmbed != nil {
			logger.Get().Warn("Failed to generate embedding for current answer, skipping cache lookup",
				zap.Error(errEmbed),
				zap.String("quizID", req.QuizID),
				zap.String("userAnswer", req.UserAnswer))
		} else {
			cachedAnswersMap, err := s.cache.HGetAll(ctx, cacheKey)
			if err == nil && len(cachedAnswersMap) > 0 {
				for _, cachedEvalDataStr := range cachedAnswersMap {
					// Call the helper method
					evaluation := s.tryGetEvaluationFromCachedItem(ctx, currentAnswerEmbedding, cachedEvalDataStr, req.QuizID, req.UserAnswer)
					if evaluation != nil {
						return evaluation, nil // Cache Hit
					}
				}
			} else if err != nil && err != domain.ErrCacheMiss {
				logger.Get().Error("Cache HGetAll failed", zap.Error(err), zap.String("key", cacheKey))
			}
		}
	}

	// 2. Original Logic: Fetch quiz, validate, call LLM (if cache miss)
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

	// 3. Cache Write Logic
	if s.cache != nil && s.cfg != nil && s.cfg.Embedding.Source != "" {
		userAnswerEmbedding, errEmbed := GenerateEmbedding(ctx, req.UserAnswer, s.cfg)
		if errEmbed != nil {
			logger.Get().Error("Failed to generate embedding for user answer, evaluation will be cached without embedding",
				zap.Error(errEmbed),
				zap.String("quizID", req.QuizID),
				zap.String("userAnswer", req.UserAnswer))
			// Decide if we still cache the response without embedding or skip.
			// For now, let's cache without embedding if generation failed.
			// The read logic already handles missing embeddings.
			// However, the task implies embedding should be stored.
			// "if errEmbed != nil, log the error and do not attempt to save this answer to the cache with an embedding"
			// This means we should not proceed to cache this specific entry if embedding fails.
		} else {
			cachedEval := CachedAnswerEvaluation{
				Evaluation: response,
				Embedding:  userAnswerEmbedding,
				UserAnswer: req.UserAnswer,
			}
			cachedJSON, errMarshal := json.Marshal(cachedEval)
			if errMarshal != nil {
				logger.Get().Error("Failed to marshal answer evaluation for caching",
					zap.Error(errMarshal),
					zap.String("quizID", req.QuizID))
			} else {
				// Using req.UserAnswer as the field key in the hash, as per original logic and task notes.
				if err := s.cache.HSet(ctx, cacheKey, req.UserAnswer, string(cachedJSON)); err != nil {
					logger.Get().Error("Failed to cache answer evaluation",
						zap.Error(err),
						zap.String("quizID", req.QuizID))
				} else if err := s.cache.Expire(ctx, cacheKey, CacheExpiration); err != nil {
					logger.Get().Error("Failed to set cache expiration",
						zap.Error(err),
						zap.String("quizID", req.QuizID))
				} else {
					logger.Get().Info("Answer evaluation and embedding cached successfully",
						zap.String("quizID", req.QuizID),
						zap.String("userAnswer", req.UserAnswer))
				}
			}
		}
	}

	return response, nil
}

// tryGetEvaluationFromCachedItem processes a single cached item and returns an evaluation if it's a valid hit.
func (s *quizService) tryGetEvaluationFromCachedItem(ctx context.Context, currentAnswerEmbedding []float32, cachedEvalDataStr string, quizIDForLookup string, userAnswerForLog string) *dto.CheckAnswerResponse {
	var cachedEntry CachedAnswerEvaluation
	errUnmarshal := json.Unmarshal([]byte(cachedEvalDataStr), &cachedEntry)
	if errUnmarshal != nil {
		logger.Get().Warn("Failed to unmarshal cached answer evaluation",
			zap.Error(errUnmarshal),
			zap.String("quizID", quizIDForLookup),
			zap.String("userAnswer", userAnswerForLog)) // Added userAnswerForLog for context
		return nil
	}

	if len(cachedEntry.Embedding) == 0 {
		logger.Get().Debug("Skipping cached entry due to missing embedding",
			zap.String("quizID", quizIDForLookup),
			zap.String("cachedUserAnswer", cachedEntry.UserAnswer),
			zap.String("userAnswer", userAnswerForLog)) // Added userAnswerForLog for context
		return nil
	}

	similarity, errSim := CosineSimilarity(currentAnswerEmbedding, cachedEntry.Embedding)
	if errSim != nil {
		logger.Get().Warn("Failed to calculate cosine similarity for cached answer",
			zap.Error(errSim),
			zap.String("quizID", quizIDForLookup),
			zap.String("userAnswer", userAnswerForLog)) // Added userAnswerForLog for context
		return nil
	}

	if similarity >= s.cfg.Embedding.SimilarityThreshold {
		logger.Get().Info("Cache hit: Found similar answer",
			zap.String("quizID", quizIDForLookup),
			zap.String("userAnswer", userAnswerForLog),
			zap.Float64("similarity", similarity),
			zap.String("cachedUserAnswer", cachedEntry.UserAnswer))

		// Update model answer from latest quiz data
		quizForModelAnswer, err := s.repo.GetQuizByID(quizIDForLookup)
		if err != nil {
			logger.Get().Error("Failed to get quiz by ID for updating model answer in cache hit",
				zap.Error(err),
				zap.String("quizID", quizIDForLookup))
			// Return the cached evaluation anyway, as updating model answer is an enhancement
			return cachedEntry.Evaluation
		}
		if quizForModelAnswer != nil { // Should ideally not be nil if no error
			cachedEntry.Evaluation.ModelAnswer = strings.Join(quizForModelAnswer.ModelAnswers, "\n")
		}
		return cachedEntry.Evaluation
	}
	return nil // No match or below threshold
}

// GetAllSubCategories implements QuizService
func (s *quizService) GetAllSubCategories() ([]string, error) {
	categories, err := s.repo.GetAllSubCategories()
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
