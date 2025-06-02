package service

import (
	"context"
	"encoding/json"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"strings"
	"time"

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
	cache        domain.Cache
	openAIAPIKey string
}

// NewQuizService creates a new instance of quizService
func NewQuizService(
	repo domain.QuizRepository,
	evaluator domain.AnswerEvaluator,
	cache domain.Cache,
	openAIAPIKey string,
) QuizService {
	return &quizService{
		repo:         repo,
		evaluator:    evaluator,
		cache:        cache,
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
	if s.cache != nil && s.openAIAPIKey != "" {
		// Try to get cached similar answer
		cachedAnswersMap, err := s.cache.HGetAll(ctx, cacheKey)
		if err == nil && len(cachedAnswersMap) > 0 {
			// Try to find similar answer in cache
			currentAnswerEmbedding, errEmbed := GenerateOpenAIEmbedding(ctx, req.UserAnswer, s.openAIAPIKey)
			if errEmbed == nil {
				for cachedUserAnswerText, cachedEvalDataStr := range cachedAnswersMap {
					cachedAnswerEmbedding, errEmbed := GenerateOpenAIEmbedding(ctx, cachedUserAnswerText, s.openAIAPIKey)
					if errEmbed != nil {
						logger.Get().Warn("Failed to generate embedding for cached answer",
							zap.Error(errEmbed),
							zap.String("cachedAnswer", cachedUserAnswerText))
						continue
					}

					similarity, errSim := CosineSimilarity(currentAnswerEmbedding, cachedAnswerEmbedding)
					if errSim != nil || similarity <= SimilarityThreshold {
						continue
					}

					// Found similar answer in cache
					var cachedEval dto.CheckAnswerResponse
					if errUnmarshal := json.Unmarshal([]byte(cachedEvalDataStr), &cachedEval); errUnmarshal == nil {
						logger.Get().Info("Cache hit: Found similar answer",
							zap.String("quizID", req.QuizID),
							zap.String("userAnswer", req.UserAnswer),
							zap.Float64("similarity", similarity))

						// Update model answer from latest quiz data
						quiz, _ := s.repo.GetQuizByID(req.QuizID)
						if quiz != nil {
							cachedEval.ModelAnswer = strings.Join(quiz.ModelAnswers, "\n")
						}
						return &cachedEval, nil
					}
				}
			}
		} else if err != nil && err != domain.ErrCacheMiss {
			logger.Get().Error("Cache lookup failed", zap.Error(err), zap.String("key", cacheKey))
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
	if s.cache != nil {
		evaluatedAnswerJSON, errMarshal := json.Marshal(response)
		if errMarshal != nil {
			logger.Get().Error("Failed to marshal LLM evaluation for caching",
				zap.Error(errMarshal),
				zap.String("quizID", req.QuizID))
		} else {
			if err := s.cache.HSet(ctx, cacheKey, req.UserAnswer, string(evaluatedAnswerJSON)); err != nil {
				logger.Get().Error("Failed to cache LLM evaluation",
					zap.Error(err),
					zap.String("quizID", req.QuizID))
			} else if err := s.cache.Expire(ctx, cacheKey, CacheExpiration); err != nil {
				logger.Get().Error("Failed to set cache expiration",
					zap.Error(err),
					zap.String("quizID", req.QuizID))
			} else {
				logger.Get().Info("LLM evaluation cached successfully",
					zap.String("quizID", req.QuizID),
					zap.String("userAnswer", req.UserAnswer))
			}
		}
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
