package domain

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
}

// QuizRepository defines the interface for quiz persistence
type QuizRepository interface {
	GetQuizByID(id string) (*Quiz, error)
	GetRandomQuiz() (*Quiz, error)
	GetRandomQuizBySubCategory(subCategory string) (*Quiz, error)
	GetSimilarQuiz(quizID string) (*Quiz, error)
	GetAllSubCategories() ([]string, error)
	SaveAnswer(answer *Answer) error
	GetQuizzesByCriteria(SubCategoryID string, limit int) ([]*Quiz, error)
	GetSubCategoryIDByName(name string) (string, error)
}

// QuizRepositoryOps provides additional repository operations (consider merging or refactoring)
type QuizRepositoryOps interface {
	GetRandomQuiz() (*Quiz, error)               // Duplicate of method in QuizRepository
	GetSimilarQuiz(quizID string) (*Quiz, error) // Duplicate of method in QuizRepository
	SaveAnswer(answer *Answer) error             // Duplicate of method in QuizRepository
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
