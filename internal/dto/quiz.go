package dto

// CategoryResponse represents a category in the API response
// @Description Category information
type CategoryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SubCategoryResponse represents a subcategory in the API response
type SubCategoryResponse struct {
	ID          string `json:"id"`
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// QuizResponse represents a quiz in the API response
// @Description Quiz information
type QuizResponse struct {
	ID           string   `json:"id"`
	Question     string   `json:"question"`
	ModelAnswers []string `json:"model_answers,omitempty"`
	Keywords     []string `json:"keywords"`
	DiffLevel    string   `json:"diff_level"`
}

// CheckAnswerRequest represents a request to check a quiz answer
// @Description Request body for checking a quiz answer
type CheckAnswerRequest struct {
	QuizID     string `json:"quiz_id" example:"ulid-generated-id"` // Quiz ID to check
	UserAnswer string `json:"user_answer" example:"Your answer"`   // User's answer text
}

// CheckAnswerResponse represents the evaluation result in the API response
type CheckAnswerResponse struct {
	Score          float64  `json:"score"`                  // Overall score (0.0 ~ 1.0)
	Explanation    string   `json:"explanation"`            // Feedback generated by LLM
	KeywordMatches []string `json:"keyword_matches"`        // Matched keywords
	Completeness   float64  `json:"completeness"`           // Answer completeness (0.0 ~ 1.0)
	Relevance      float64  `json:"relevance"`              // Answer relevance (0.0 ~ 1.0)
	Accuracy       float64  `json:"accuracy"`               // Answer accuracy (0.0 ~ 1.0)
	ModelAnswer    string   `json:"model_answer,omitempty"` // Model answer (optional)
}

// QuizEvaluationResponse represents the evaluation criteria in the API response
type QuizEvaluationResponse struct {
	ScoreRange  string `json:"score_range"`
	Explanation string `json:"explanation"`
}

// ErrorResponse represents an error in the API response
type ErrorResponse struct {
	Error string `json:"error"`
}

// BulkQuizzesRequest represents a request to get bulk quizzes
// @Description Request query parameters for getting bulk quizzes
type BulkQuizzesRequest struct {
	SubCategory string `query:"sub_category" validate:"required"`        // Sub-category of the quizzes
	Count       int    `query:"count" validate:"omitempty,gte=1,lte=50"` // Number of quizzes to fetch (default: 10)
}

// BulkQuizzesResponse represents a list of quizzes in the API response
// @Description Response body for a list of quizzes
type BulkQuizzesResponse struct {
	Quizzes []QuizResponse `json:"quizzes"`
}

// BulkQuizResponse represents multiple quizzes in the API response
type BulkQuizResponse struct {
	Quizzes     []QuizResponse `json:"quizzes"`
	Total       int            `json:"total"`
	PageSize    int            `json:"page_size"`
	CurrentPage int            `json:"current_page"`
}

// SubCategoryIDsResponse represents a list of subcategory IDs
// @Description Response body for a list of subcategory IDs
type SubCategoryIDsResponse struct {
	SubCategoryIDs []string `json:"sub_category_ids"`
}
