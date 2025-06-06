package domain

import (
	"context"
	"quiz-byte/internal/dto" // Added for QuizRecommendationItem
)

// TransactionManager는 트랜잭션을 관리하는 도메인 인터페이스
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// QuizService defines the core business operations for quizzes
type QuizService interface {
	// GetRandomQuiz returns a random quiz from the specified subcategory
	GetRandomQuiz(subCategoryID string) (*Quiz, error)

	// CheckAnswer evaluates a user's answer to a quiz
	CheckAnswer(quizID string, userAnswer string) (*Answer, error)

	// GetNextQuiz returns a similar quiz based on keywords
	GetNextQuiz(currentQuizID string) (*Quiz, error)

	// GetAllCategories returns all categories with their subcategories
	GetAllCategories() ([]*Category, error)

	// GetSubCategories returns all subcategories for a given category
	GetSubCategories(categoryID string) ([]*SubCategory, error)

	InvalidateQuizCache(ctx context.Context, quizID string) error
}

// QuizRepository defines the interface for quiz persistence
type QuizRepository interface {
	GetQuizByID(ctx context.Context, id string) (*Quiz, error)
	GetRandomQuiz(ctx context.Context) (*Quiz, error)
	GetRandomQuizBySubCategory(ctx context.Context, subCategory string) (*Quiz, error)
	GetSimilarQuiz(ctx context.Context, quizID string) (*Quiz, error)
	GetAllSubCategories(ctx context.Context) ([]string, error)
	SaveAnswer(ctx context.Context, answer *Answer) error
	SaveQuiz(ctx context.Context, quiz *Quiz) error
	GetQuizzesByCriteria(ctx context.Context, SubCategoryID string, limit int) ([]*Quiz, error)
	GetSubCategoryIDByName(ctx context.Context, name string) (string, error)
	GetQuizzesBySubCategory(ctx context.Context, subCategoryID string) ([]*Quiz, error)
	// Methods from internal/domain/quiz.go
	UpdateQuiz(ctx context.Context, quiz *Quiz) error
	SaveQuizEvaluation(ctx context.Context, evaluation *QuizEvaluation) error
	GetQuizEvaluation(ctx context.Context, quizID string) (*QuizEvaluation, error)
	GetUnattemptedQuizzesWithDetails(ctx context.Context, userID string, limit int, optionalSubCategoryID string) ([]dto.QuizRecommendationItem, error)
}

// CategoryRepository defines the interface for category persistence
type CategoryRepository interface {
	// GetAllCategories returns all categories
	GetAllCategories(ctx context.Context) ([]*Category, error)

	// GetSubCategories returns all subcategories for a given category
	GetSubCategories(ctx context.Context, categoryID string) ([]*SubCategory, error)

	// SaveCategory persists a new category
	SaveCategory(ctx context.Context, category *Category) error

	// SaveSubCategory persists a new subcategory
	SaveSubCategory(ctx context.Context, subCategory *SubCategory) error

	GetByName(ctx context.Context, name string) (*Category, error)
	GetByNameAndCategoryID(ctx context.Context, name string, categoryID string) (*SubCategory, error)
}
