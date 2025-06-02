package service

import (
	"context"
	"encoding/json"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	QuizAnswerCachePrefix = "quizanswers:"
	SimilarityThreshold   = 0.95 // Example threshold
	CacheExpiration       = 24 * time.Hour
	EmbeddingVectorSize   = 1536 // OpenAI Ada v2 embeddings are 1536 dimensional
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
	repo         domain.QuizRepository
	evaluator    domain.AnswerEvaluator
	redisClient  *redis.Client
	openAIAPIKey string
}

// NewQuizService creates a new instance of quizService
func NewQuizService(
	repo domain.QuizRepository,
	evaluator domain.AnswerEvaluator,
	redisClient *redis.Client,
	openAIAPIKey string,
) QuizService {
	return &quizService{
		repo:         repo,
		evaluator:    evaluator,
		redisClient:  redisClient,
		openAIAPIKey: openAIAPIKey,
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
	if s.redisClient != nil && s.openAIAPIKey != "" { // Only attempt cache read if Redis and API key are available
		cachedAnswersMap, err := s.redisClient.HGetAll(ctx, cacheKey).Result()
		if err == nil && len(cachedAnswersMap) > 0 { // Cache hit on the Hash key itself
			currentAnswerEmbedding, errEmbed := GenerateOpenAIEmbedding(ctx, req.UserAnswer, s.openAIAPIKey)
			if errEmbed != nil {
				logger.Get().Error("Failed to generate embedding for current answer", zap.Error(errEmbed), zap.String("quizID", req.QuizID))
				// Proceed as cache miss if embedding fails
			} else {
				for cachedUserAnswerText, cachedEvalDataStr := range cachedAnswersMap {
					// Note: Storing raw text and re-generating embeddings for cached answers on each check is inefficient.
					// A better approach would be to store embeddings alongside the answer, or use a vector DB.
					// For this exercise, we follow the described path of re-generating.
					cachedAnswerEmbedding, errEmbedCached := GenerateOpenAIEmbedding(ctx, cachedUserAnswerText, s.openAIAPIKey)
					if errEmbedCached != nil {
						logger.Get().Warn("Failed to generate embedding for cached answer", zap.Error(errEmbedCached), zap.String("cachedAnswer", cachedUserAnswerText))
						continue // Skip this cached item
					}

					similarity, errSim := CosineSimilarity(currentAnswerEmbedding, cachedAnswerEmbedding)
					if errSim != nil {
						logger.Get().Warn("Failed to calculate cosine similarity", zap.Error(errSim), zap.String("quizID", req.QuizID))
						continue // Skip this cached item
					}

					logger.Get().Debug("Calculated similarity", zap.Float64("similarity", similarity), zap.String("quizID", req.QuizID), zap.String("userAnswer", req.UserAnswer), zap.String("cachedAnswer", cachedUserAnswerText))

					if similarity > SimilarityThreshold {
						var cachedEval dto.CheckAnswerResponse
						if errUnmarshal := json.Unmarshal([]byte(cachedEvalDataStr), &cachedEval); errUnmarshal == nil {
							logger.Get().Info("Cache hit: Found similar answer", zap.String("quizID", req.QuizID), zap.String("userAnswer", req.UserAnswer), zap.Float64("similarity", similarity))
							quizForModelAnswer, _ := s.repo.GetQuizByID(req.QuizID)
							if quizForModelAnswer != nil {
								cachedEval.ModelAnswer = strings.Join(quizForModelAnswer.ModelAnswers, "\n")
							}
							logger.Get().Warn("Failed to unmarshal cached evaluation data", zap.Error(errUnmarshal), zap.String("quizID", req.QuizID))
							return &cachedEval, nil
						}

					}
				}
			}
		} else if err != nil && err != redis.Nil { // redis.Nil means key doesn't exist, which is a valid cache miss.
			logger.Get().Error("Redis HGetAll failed", zap.Error(err), zap.String("cacheKey", cacheKey))
			// Proceed as cache miss
		}
	} else {
		if s.redisClient == nil {
			logger.Get().Warn("Redis client not initialized, skipping cache read.")
		}
		if s.openAIAPIKey == "" {
			logger.Get().Warn("OpenAI API key not configured, skipping cache read based on embeddings.")
		}
	}

	logger.Get().Info("Cache miss: No similar answer found in cache or error in cache lookup", zap.String("quizID", req.QuizID), zap.String("userAnswer", req.UserAnswer))

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
	if s.redisClient != nil {
		evaluatedAnswerJSON, errMarshal := json.Marshal(response)
		if errMarshal != nil {
			logger.Get().Error("Failed to marshal LLM evaluation for caching", zap.Error(errMarshal), zap.String("quizID", req.QuizID))
		} else {
			errCacheSet := s.redisClient.HSet(ctx, cacheKey, req.UserAnswer, string(evaluatedAnswerJSON)).Err()
			if errCacheSet != nil {
				logger.Get().Error("Failed to cache LLM evaluation (HSet)", zap.Error(errCacheSet), zap.String("quizID", req.QuizID))
			} else {
				// Set expiration for the hash key
				errExpire := s.redisClient.Expire(ctx, cacheKey, CacheExpiration).Err()
				if errExpire != nil {
					logger.Get().Error("Failed to set cache expiration", zap.Error(errExpire), zap.String("quizID", req.QuizID))
				} else {
					logger.Get().Info("LLM evaluation cached successfully", zap.String("quizID", req.QuizID), zap.String("userAnswer", req.UserAnswer))
				}
			}
		}
	} else {
		logger.Get().Warn("Redis client not initialized, skipping cache write.")
	}

	return response, nil
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
