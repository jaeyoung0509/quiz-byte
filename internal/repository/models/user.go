package models

import (
	"database/sql"
	"time"
)

// User represents a user in the system.
type User struct {
	ID                  string       `db:"id"`                     // ULID
	GoogleID            string       `db:"google_id"`              // Google's unique identifier for the user
	Email               string       `db:"email"`                  // User's email address
	Name                sql.NullString `db:"name"`                   // User's full name
	ProfilePictureURL   sql.NullString `db:"profile_picture_url"`  // URL of the user's profile picture
	EncryptedAccessToken sql.NullString `db:"encrypted_access_token"` // Encrypted Google OAuth access token
	EncryptedRefreshToken sql.NullString `db:"encrypted_refresh_token"`// Encrypted Google OAuth refresh token
	TokenExpiresAt      sql.NullTime `db:"token_expires_at"`       // Expiry time for the access token
	CreatedAt           time.Time    `db:"created_at"`             // Timestamp of user creation
	UpdatedAt           time.Time    `db:"updated_at"`             // Timestamp of last update
	DeletedAt           sql.NullTime `db:"deleted_at"`             // Timestamp of soft deletion, if applicable
}

// UserQuizAttempt represents a user's attempt at a quiz.
type UserQuizAttempt struct {
	ID                 string        `db:"id"`                    // ULID
	UserID             string        `db:"user_id"`               // Foreign key to users table
	QuizID             string        `db:"quiz_id"`               // Foreign key to quizzes table
	UserAnswer         sql.NullString  `db:"user_answer"`           // User's submitted answer
	LlmScore           sql.NullFloat64 `db:"llm_score"`             // Score given by the LLM
	LlmExplanation     sql.NullString  `db:"llm_explanation"`       // Explanation from the LLM
	LlmKeywordMatches  StringSlice   `db:"llm_keyword_matches"`   // Keywords matched, using existing StringSlice type
	LlmCompleteness    sql.NullFloat64 `db:"llm_completeness"`      // Completeness score from LLM
	LlmRelevance       sql.NullFloat64 `db:"llm_relevance"`         // Relevance score from LLM
	LlmAccuracy        sql.NullFloat64 `db:"llm_accuracy"`          // Accuracy score from LLM
	IsCorrect          bool          `db:"is_correct"`            // Whether the answer was deemed correct (e.g., score >= threshold)
	AttemptedAt        time.Time     `db:"attempted_at"`          // Timestamp when the attempt was made
	CreatedAt          time.Time     `db:"created_at"`            // Timestamp of record creation
	UpdatedAt          time.Time     `db:"updated_at"`            // Timestamp of last update
	DeletedAt          sql.NullTime  `db:"deleted_at"`            // Timestamp of soft deletion, if applicable
}

// TableName methods to satisfy potential ORM expectations, though sqlx doesn't strictly need them.
