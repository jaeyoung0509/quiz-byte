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
	QuizRepositoryOps

	// GetRandomQuizBySubCategory returns a random quiz from the specified subcategory
	GetRandomQuizBySubCategory(subCategoryID string) (*Quiz, error)

	// GetQuizByID retrieves a quiz by its ID
	GetQuizByID(id string) (*Quiz, error)

	// GetAllSubCategories returns all available subcategories
	GetAllSubCategories() ([]string, error)

	// SaveQuiz persists a new quiz
	SaveQuiz(quiz *Quiz) error

	// UpdateQuiz updates an existing quiz
	UpdateQuiz(quiz *Quiz) error

	// SaveQuizEvaluation persists quiz evaluation criteria
	SaveQuizEvaluation(evaluation *QuizEvaluation) error

	// GetQuizEvaluation retrieves evaluation criteria for a quiz
	GetQuizEvaluation(quizID string) (*QuizEvaluation, error)
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
