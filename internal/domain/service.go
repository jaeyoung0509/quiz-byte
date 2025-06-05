package domain

import "context"

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
	GetQuizByID(id string) (*Quiz, error)
	GetRandomQuiz() (*Quiz, error)
	GetRandomQuizBySubCategory(subCategory string) (*Quiz, error)
	GetSimilarQuiz(quizID string) (*Quiz, error)
	GetAllSubCategories(ctx context.Context) ([]string, error)
	SaveAnswer(answer *Answer) error // Assuming SaveAnswer might also need context later, but not in scope for this change
	SaveQuiz(ctx context.Context, quiz *Quiz) error
	GetQuizzesByCriteria(SubCategoryID string, limit int) ([]*Quiz, error)
	GetSubCategoryIDByName(name string) (string, error)
	GetQuizzesBySubCategory(ctx context.Context, subCategoryID string) ([]*Quiz, error)
}

// CategoryRepository defines the interface for category persistence
type CategoryRepository interface {
	// GetAllCategories returns all categories
	GetAllCategories() ([]*Category, error)

	// GetSubCategories returns all subcategories for a given category
	GetSubCategories(categoryID string) ([]*SubCategory, error)

	// SaveCategory persists a new category
	SaveCategory(category *Category) error

	// SaveSubCategory persists a new subcategory
	SaveSubCategory(subCategory *SubCategory) error
}
