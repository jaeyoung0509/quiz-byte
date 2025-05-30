package domain

import (
	"fmt"
	"strings"
	"time"
)

// ValidationError represents a validation error
type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return e.message
}

func NewValidationError(message string) error {
	return &ValidationError{message: message}
}

// NotFoundError represents a not found error
type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return e.message
}

// Category represents a quiz category
type Category struct {
	ID            int64
	Name          string
	Description   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SubCategories []*SubCategory
}

// NewCategory creates a new Category instance
func NewCategory(name, description string) *Category {
	now := time.Now()
	return &Category{
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate validates the category
func (c *Category) Validate() error {
	if c.Name == "" {
		return NewValidationError("name is required")
	}
	return nil
}

// SubCategory represents a subcategory under a main category
type SubCategory struct {
	ID          int64
	CategoryID  int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewSubCategory creates a new SubCategory instance
func NewSubCategory(categoryID int64, name, description string) *SubCategory {
	now := time.Now()
	return &SubCategory{
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate validates the subcategory
func (s *SubCategory) Validate() error {
	if s.CategoryID == 0 {
		return NewValidationError("category ID is required")
	}
	if s.Name == "" {
		return NewValidationError("name is required")
	}
	return nil
}

// Quiz represents a quiz in the domain
type Quiz struct {
	ID           int64
	Question     string
	ModelAnswers []string // 모범 답안들
	Keywords     []string // 유사도 매칭용 키워드
	Difficulty   int      // 난이도 (1: Easy, 2: Medium, 3: Hard)
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewQuiz creates a new Quiz instance
func NewQuiz(question string, modelAnswers []string, keywords []string, difficulty int) *Quiz {
	now := time.Now()
	return &Quiz{
		Question:     question,
		ModelAnswers: modelAnswers,
		Keywords:     keywords,
		Difficulty:   difficulty,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// DifficultyToString converts the difficulty level to a string representation
func DifficultyToInt(diff string) int {
	switch strings.ToLower(diff) {
	case "easy":
		return 1
	case "medium":
		return 2
	case "hard":
		return 3
	default:
		return 1
	}
}

func (q *Quiz) DifficultyToString() string {
	switch q.Difficulty {
	case 1:
		return "easy"
	case 2:
		return "medium"
	case 3:
		return "hard"
	default:
		return "easy"
	}
}

// Validate validates the quiz
func (q *Quiz) Validate() error {
	if q.Question == "" {
		return NewValidationError("question is required")
	}
	if len(q.ModelAnswers) == 0 {
		return NewValidationError("at least one model answer is required")
	}
	return nil
}

// Answer represents a user's answer to a quiz
type Answer struct {
	ID             int64
	QuizID         int64
	UserAnswer     string   // 서술형 답변
	Score          float64  // 0.0 ~ 1.0 사이의 점수
	Explanation    string   // LLM이 생성한 피드백
	KeywordMatches []string // 매칭된 키워드들
	Completeness   float64  // 답변 완성도 (0.0 ~ 1.0)
	Relevance      float64  // 답변 관련성 (0.0 ~ 1.0)
	Accuracy       float64  // 답변 정확도 (0.0 ~ 1.0)
	AnsweredAt     time.Time
}

// NewAnswer creates a new Answer instance
func NewAnswer(quizID int64, userAnswer string) *Answer {
	return &Answer{
		QuizID:     quizID,
		UserAnswer: userAnswer,
		AnsweredAt: time.Now(),
	}
}

// Validate validates the answer
func (a *Answer) Validate() error {
	if a.QuizID == 0 {
		return NewValidationError("quiz ID is required")
	}
	if a.UserAnswer == "" {
		return NewValidationError("user answer is required")
	}
	return nil
}

// QuizEvaluation represents the evaluation criteria for a quiz
type QuizEvaluation struct {
	ID              int64
	QuizID          int64
	MinimumKeywords int      // 최소 필요 키워드 수
	RequiredTopics  []string // 필수 포함 주제들
	ScoreRanges     []string // 점수 범위별 기준
	SampleAnswers   []string // 예시 답안들
	RubricDetails   string   // 채점 기준 상세 설명
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewQuizEvaluation creates a new QuizEvaluation instance
func NewQuizEvaluation(
	quizID int64,
	minKeywords int,
	requiredTopics []string,
	sampleAnswers []string,
	rubricDetails string,
) *QuizEvaluation {
	now := time.Now()
	return &QuizEvaluation{
		QuizID:          quizID,
		MinimumKeywords: minKeywords,
		RequiredTopics:  requiredTopics,
		SampleAnswers:   sampleAnswers,
		RubricDetails:   rubricDetails,
		ScoreRanges:     []string{"0-0.3", "0.3-0.6", "0.6-0.8", "0.8-1.0"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Validate validates the quiz evaluation
func (e *QuizEvaluation) Validate() error {
	if e.QuizID == 0 {
		return NewValidationError("quiz ID is required")
	}
	if e.MinimumKeywords == 0 {
		return NewValidationError("minimum keywords is required")
	}
	if len(e.RequiredTopics) == 0 {
		return NewValidationError("required topics are required")
	}
	if len(e.ScoreRanges) == 0 {
		return NewValidationError("score ranges are required")
	}
	if len(e.SampleAnswers) == 0 {
		return NewValidationError("sample answers are required")
	}
	if e.RubricDetails == "" {
		return NewValidationError("rubric details are required")
	}
	return nil
}

// InternalError represents an internal error
type InternalError struct {
	message string
	cause   error
}

func (e *InternalError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// InvalidCategoryError represents an invalid category error
type InvalidCategoryError struct {
	category string
}

func (e *InvalidCategoryError) Error() string {
	return fmt.Sprintf("invalid category: %s", e.category)
}

// QuizNotFoundError represents a quiz not found error
type QuizNotFoundError struct {
	quizID int64
}

func (e *QuizNotFoundError) Error() string {
	return fmt.Sprintf("quiz not found: %d", e.quizID)
}

// LLMServiceError represents an LLM service error
type LLMServiceError struct {
	cause error
}

func (e *LLMServiceError) Error() string {
	return fmt.Sprintf("LLM service error: %v", e.cause)
}

// QuizRepositoryOps provides additional repository operations
type QuizRepositoryOps interface {
	GetRandomQuiz() (*Quiz, error)
	GetSimilarQuiz(quizID int64) (*Quiz, error)
	SaveAnswer(answer *Answer) error
}
