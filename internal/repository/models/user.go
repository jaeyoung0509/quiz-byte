package models

import (
	"database/sql"
	"time"
)

// User represents a user in the system.
type User struct {
	ID                    string         `db:"ID"`                      // ULID
	GoogleID              string         `db:"GOOGLE_ID"`               // Google's unique identifier for the user
	Email                 string         `db:"EMAIL"`                   // User's email address
	Name                  sql.NullString `db:"NAME"`                    // User's full name
	ProfilePictureURL     sql.NullString `db:"PROFILE_PICTURE_URL"`     // URL of the user's profile picture
	EncryptedAccessToken  sql.NullString `db:"ENCRYPTED_ACCESS_TOKEN"`  // Encrypted Google OAuth access token
	EncryptedRefreshToken sql.NullString `db:"ENCRYPTED_REFRESH_TOKEN"` // Encrypted Google OAuth refresh token
	TokenExpiresAt        sql.NullTime   `db:"TOKEN_EXPIRES_AT"`        // Expiry time for the access token
	CreatedAt             time.Time      `db:"CREATED_AT"`              // Timestamp of user creation
	UpdatedAt             time.Time      `db:"UPDATED_AT"`              // Timestamp of last update
	DeletedAt             sql.NullTime   `db:"DELETED_AT"`              // Timestamp of soft deletion, if applicable
}

// UserQuizAttempt represents a user's attempt at a quiz.
type UserQuizAttempt struct {
	ID                string          `db:"ID"`                  // ULID
	UserID            string          `db:"USER_ID"`             // Foreign key to users table
	QuizID            string          `db:"QUIZ_ID"`             // Foreign key to quizzes table
	UserAnswer        sql.NullString  `db:"USER_ANSWER"`         // User's submitted answer
	LlmScore          sql.NullFloat64 `db:"LLM_SCORE"`           // Score given by the LLM
	LlmExplanation    sql.NullString  `db:"LLM_EXPLANATION"`     // Explanation from the LLM
	LlmKeywordMatches StringSlice     `db:"LLM_KEYWORD_MATCHES"` // Keywords matched, using existing StringSlice type
	LlmCompleteness   sql.NullFloat64 `db:"LLM_COMPLETENESS"`    // Completeness score from LLM
	LlmRelevance      sql.NullFloat64 `db:"LLM_RELEVANCE"`       // Relevance score from LLM
	LlmAccuracy       sql.NullFloat64 `db:"LLM_ACCURACY"`        // Accuracy score from LLM
	IsCorrect         bool            `db:"IS_CORRECT"`          // Whether the answer was deemed correct (e.g., score >= threshold)
	AttemptedAt       time.Time       `db:"ATTEMPTED_AT"`        // Timestamp when the attempt was made
	CreatedAt         time.Time       `db:"CREATED_AT"`          // Timestamp of record creation
	UpdatedAt         time.Time       `db:"UPDATED_AT"`          // Timestamp of last update
	DeletedAt         sql.NullTime    `db:"DELETED_AT"`          // Timestamp of soft deletion, if applicable
}

// TableName methods to satisfy potential ORM expectations, though sqlx doesn't strictly need them.
