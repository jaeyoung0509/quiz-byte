package domain

import (
	"context"
	"time"

	"quiz-byte/internal/dto"
)

// User represents a domain user object
type User struct {
	ID                string
	GoogleID          string
	Email             string
	Name              string
	ProfilePictureURL string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// NewUser creates a new User instance
func NewUser(googleID, email string) *User {
	now := time.Now()
	return &User{
		GoogleID:  googleID,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate validates the user
func (u *User) Validate() error {
	if u.GoogleID == "" {
		return NewValidationError("google_id is required")
	}
	if u.Email == "" {
		return NewValidationError("email is required")
	}
	return nil
}

// UserQuizAttempt represents a user's attempt at a quiz question.
type UserQuizAttempt struct {
	ID                string
	UserID            string
	QuizID            string
	UserAnswer        string // Assuming string, can be *string if nullable
	LLMScore          float64
	LLMExplanation    string // Assuming string, can be *string if nullable
	LLMKeywordMatches []string
	LLMCompleteness   float64
	LLMRelevance      float64
	LLMAccuracy       float64
	IsCorrect         bool
	AttemptedAt       time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// UserRepository defines the interface for user data persistence.
type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByGoogleID(ctx context.Context, googleID string) (*User, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
}

// UserQuizAttemptRepository defines the interface for user quiz attempt data persistence.
type UserQuizAttemptRepository interface {
	CreateAttempt(ctx context.Context, attempt *UserQuizAttempt) error
	GetAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]UserQuizAttempt, int, error)
	GetIncorrectAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]UserQuizAttempt, int, error)
}
