package service

import (
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"strings"
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
}

// quizService implements QuizService
type quizService struct {
	repo      domain.QuizRepository
	evaluator domain.AnswerEvaluator
}

// NewQuizService creates a new instance of quizService
func NewQuizService(
	repo domain.QuizRepository,
	evaluator domain.AnswerEvaluator,
) QuizService {
	return &quizService{
		repo:      repo,
		evaluator: evaluator,
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

	// Find next similar quiz
	nextQuiz, err := s.repo.GetSimilarQuiz(req.QuizID)
	var nextQuizID int64
	if err == nil && nextQuiz != nil {
		nextQuizID = nextQuiz.ID
	}

	return &dto.CheckAnswerResponse{
		Score:          evaluatedAnswer.Score,
		Explanation:    evaluatedAnswer.Explanation,
		KeywordMatches: evaluatedAnswer.KeywordMatches,
		Completeness:   evaluatedAnswer.Completeness,
		Relevance:      evaluatedAnswer.Relevance,
		Accuracy:       evaluatedAnswer.Accuracy,
		ModelAnswer:    strings.Join(quiz.ModelAnswers, "\n"), // 모든 모범답안을 줄바꿈으로 구분하여 표시
		NextQuizID:     nextQuizID,
	}, nil
}

// GetAllSubCategories implements QuizService
func (s *quizService) GetAllSubCategories() ([]string, error) {
	categories, err := s.repo.GetAllSubCategories()
	if err != nil {
		return nil, domain.NewInternalError("Failed to get subcategories", err)
	}
	return categories, nil
}
