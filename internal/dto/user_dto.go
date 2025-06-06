package dto

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GoogleUserInfo holds user information obtained from Google.
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// AuthClaims defines the custom claims for JWT.
type AuthClaims struct {
	UserID    string `json:"user_id"`
	TokenType string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// UserProfileResponse defines the structure for a user's profile information.
type UserProfileResponse struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Name              string `json:"name,omitempty"`
	ProfilePictureURL string `json:"profile_picture_url,omitempty"`
}

// TokenResponse represents the response containing access and refresh tokens.
// @Description Response body for authentication tokens
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenRequest represents the request body for refreshing a token.
// @Description Request body for refreshing JWT tokens
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// MessageResponse represents a generic message response.
// @Description Generic message response
type MessageResponse struct {
	Message string `json:"message"`
}

// --- Pagination and Filtering DTOs ---

// Pagination defines parameters for paginated requests.
// These are typically query parameters.
type Pagination struct {
	Limit  int `query:"limit"`  // Number of items per page
	Offset int `query:"offset"` // Number of items to skip
	Page   int `query:"page"`   // Page number (alternative to offset)
}

// PaginationInfo defines pagination details for responses.
type PaginationInfo struct {
	TotalItems  int64 `json:"total_items"`
	Limit       int   `json:"limit"`
	Offset      int   `json:"offset"`
	CurrentPage int   `json:"current_page"`
	TotalPages  int   `json:"total_pages"`
}

// AttemptFilters defines parameters for filtering lists of quiz attempts.
// These are typically query parameters.
type AttemptFilters struct {
	CategoryID string `query:"category_id"` // Can be main or sub-category ID
	StartDate  string `query:"start_date"`  // Format: YYYY-MM-DD
	EndDate    string `query:"end_date"`    // Format: YYYY-MM-DD
	IsCorrect  *bool  `query:"is_correct"`  // Pointer for tri-state: true, false, or omit for no filter
	SortBy     string `query:"sort_by"`     // e.g., "attempted_at", "score"
	SortOrder  string `query:"sort_order"`  // "ASC" or "DESC"
}

// --- User Quiz Attempts DTOs ---

// UserQuizAttemptItem represents a single quiz attempt in a list.
type UserQuizAttemptItem struct {
	AttemptID      string    `json:"attempt_id"`
	QuizID         string    `json:"quiz_id"`
	QuizQuestion   string    `json:"quiz_question"`
	UserAnswer     string    `json:"user_answer"`
	LlmScore       float64   `json:"llm_score"`
	LlmExplanation string    `json:"llm_explanation,omitempty"`
	IsCorrect      bool      `json:"is_correct"`
	AttemptedAt    time.Time `json:"attempted_at"`
	// Add more LLM fields if needed: LlmKeywordMatches, LlmCompleteness, etc.
}

// UserQuizAttemptsResponse is the response for listing user quiz attempts.
type UserQuizAttemptsResponse struct {
	Attempts       []UserQuizAttemptItem `json:"attempts"`
	PaginationInfo PaginationInfo        `json:"pagination_info"`
}

// --- User Incorrect Answers DTOs ---

// UserIncorrectAnswerItem represents a single incorrect answer for the user.
type UserIncorrectAnswerItem struct {
	AttemptID      string    `json:"attempt_id"`
	QuizID         string    `json:"quiz_id"`
	QuizQuestion   string    `json:"quiz_question"`
	UserAnswer     string    `json:"user_answer"`
	CorrectAnswer  string    `json:"correct_answer"`            // Model answer from the quiz
	LlmScore       float64   `json:"llm_score"`                 // User's score on their attempt
	LlmExplanation string    `json:"llm_explanation,omitempty"` // LLM's explanation for user's answer
	AttemptedAt    time.Time `json:"attempted_at"`
	// QuizExplanation string `json:"quiz_explanation,omitempty"` // If quizzes have a general explanation
}

// UserIncorrectAnswersResponse is the response for listing user's incorrect answers.
type UserIncorrectAnswersResponse struct {
	IncorrectAnswers []UserIncorrectAnswerItem `json:"incorrect_answers"`
	PaginationInfo   PaginationInfo            `json:"pagination_info"`
}

// --- Quiz Recommendation DTOs (Basic) ---

// QuizRecommendationItem represents a single recommended quiz.
type QuizRecommendationItem struct {
	QuizID          string `json:"quiz_id"`
	QuizQuestion    string `json:"quiz_question"`
	SubCategoryName string `json:"sub_category_name,omitempty"` // Or full category path
	Difficulty      int    `json:"difficulty,omitempty"`
}

// QuizRecommendationsResponse is the response for listing recommended quizzes.
type QuizRecommendationsResponse struct {
	Recommendations []QuizRecommendationItem `json:"recommendations"`
	// Context string `json:"context,omitempty"` // Future: why these were recommended
}

// AuthenticatedUser represents the user data returned upon successful authentication
// by the AuthService, intended for internal use before constructing the final HTTP response.
type AuthenticatedUser struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Name              string `json:"name,omitempty"`
	ProfilePictureURL string `json:"profile_picture_url,omitempty"`
}
