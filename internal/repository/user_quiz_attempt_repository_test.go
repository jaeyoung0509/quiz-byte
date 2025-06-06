package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/repository/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// setupUserQuizAttemptTestDB creates a new sqlx.DB instance and sqlmock for user quiz attempt repository testing.
func setupUserQuizAttemptTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return sqlxDB, mock
}

// --- Tests for Converter Functions ---

func TestToDomainUserQuizAttempt(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	modelAttempt := &models.UserQuizAttempt{
		ID:                "attempt1",
		UserID:            "user1",
		QuizID:            "quiz1",
		UserAnswer:        sql.NullString{String: "My answer", Valid: true},
		LlmScore:          sql.NullFloat64{Float64: 0.8, Valid: true},
		LlmExplanation:    sql.NullString{String: "Good", Valid: true},
		LlmKeywordMatches: models.StringSlice{"key1", "key2"},
		LlmCompleteness:   sql.NullFloat64{Float64: 0.9, Valid: true},
		LlmRelevance:      sql.NullFloat64{Float64: 0.7, Valid: true},
		LlmAccuracy:       sql.NullFloat64{Float64: 0.85, Valid: true},
		IsCorrect:         true,
		AttemptedAt:       now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	domainAttempt := toDomainUserQuizAttempt(modelAttempt)
	assert.NotNil(t, domainAttempt)
	assert.Equal(t, modelAttempt.ID, domainAttempt.ID)
	assert.Equal(t, modelAttempt.UserAnswer.String, domainAttempt.UserAnswer)
	assert.Equal(t, modelAttempt.LlmScore.Float64, domainAttempt.LLMScore)
	assert.EqualValues(t, modelAttempt.LlmKeywordMatches, domainAttempt.LLMKeywordMatches)
	// ... other fields

	// Test with null values
	modelAttempt.UserAnswer.Valid = false
	modelAttempt.LlmScore.Valid = false
	domainAttempt = toDomainUserQuizAttempt(modelAttempt)
	assert.Equal(t, "", domainAttempt.UserAnswer)
	assert.Equal(t, 0.0, domainAttempt.LLMScore)

	assert.Nil(t, toDomainUserQuizAttempt(nil))
}

func TestFromDomainUserQuizAttempt(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	domainAttempt := &domain.UserQuizAttempt{
		ID:                "attempt1",
		UserID:            "user1",
		QuizID:            "quiz1",
		UserAnswer:        "My answer",
		LLMScore:          0.8,
		LLMExplanation:    "Good",
		LLMKeywordMatches: []string{"key1", "key2"},
		LLMCompleteness:   0.9,
		LLMRelevance:      0.7,
		LLMAccuracy:       0.85,
		IsCorrect:         true,
		AttemptedAt:       now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	modelAttempt := fromDomainUserQuizAttempt(domainAttempt)
	assert.NotNil(t, modelAttempt)
	assert.Equal(t, domainAttempt.ID, modelAttempt.ID)
	assert.Equal(t, domainAttempt.UserAnswer, modelAttempt.UserAnswer.String)
	assert.True(t, modelAttempt.UserAnswer.Valid)
	assert.Equal(t, domainAttempt.LLMScore, modelAttempt.LlmScore.Float64)
	assert.True(t, modelAttempt.LlmScore.Valid) // Assuming 0 is a valid score and should be set
	assert.EqualValues(t, domainAttempt.LLMKeywordMatches, modelAttempt.LlmKeywordMatches)
	// ... other fields

	// Test with zero values for score (should be considered valid if not explicitly handled)
	domainAttempt.LLMScore = 0
	modelAttempt = fromDomainUserQuizAttempt(domainAttempt)
	assert.True(t, modelAttempt.LlmScore.Valid, "LlmScore should be valid even if 0, unless 0 means null")
	assert.Equal(t, 0.0, modelAttempt.LlmScore.Float64)

	assert.Nil(t, fromDomainUserQuizAttempt(nil))
}

// --- Tests for Adapter Methods ---

func TestSQLXUserQuizAttemptRepository_CreateAttempt_Success(t *testing.T) {
	db, mock := setupUserQuizAttemptTestDB(t)
	repo := NewSQLXUserQuizAttemptRepository(db)
	defer db.Close()

	attempt := &domain.UserQuizAttempt{
		ID:     "attempt-id-123", // Usually set by adapter, but pre-set for test predictability
		UserID: "user-id-456",
		QuizID: "quiz-id-789",
		// ... other fields can be set as needed
	}

	// Regex for INSERT INTO user_quiz_attempts ... VALUES ...
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_quiz_attempts`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateAttempt(context.Background(), attempt)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLXUserQuizAttemptRepository_GetAttemptsByUserID_Success(t *testing.T) {
	db, mock := setupUserQuizAttemptTestDB(t)
	repo := NewSQLXUserQuizAttemptRepository(db)
	defer db.Close()

	userID := "user-test-id"
	now := time.Now()
	expectedModels := []models.UserQuizAttempt{
		{ID: "attempt1", UserID: userID, QuizID: "q1", UserAnswer: sql.NullString{String: "Ans1", Valid: true}, AttemptedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "attempt2", UserID: userID, QuizID: "q2", UserAnswer: sql.NullString{String: "Ans2", Valid: true}, AttemptedAt: now, CreatedAt: now, UpdatedAt: now},
	}

	rows := sqlmock.NewRows([]string{"id", "user_id", "quiz_id", "user_answer", "llm_score", "llm_explanation", "llm_keyword_matches", "llm_completeness", "llm_relevance", "llm_accuracy", "is_correct", "attempted_at", "created_at", "updated_at", "deleted_at"})
	for _, ma := range expectedModels {
		rows.AddRow(ma.ID, ma.UserID, ma.QuizID, ma.UserAnswer, ma.LlmScore, ma.LlmExplanation, ma.LlmKeywordMatches, ma.LlmCompleteness, ma.LlmRelevance, ma.LlmAccuracy, ma.IsCorrect, ma.AttemptedAt, ma.CreatedAt, ma.UpdatedAt, ma.DeletedAt)
	}

	// Simplified regex for the results query
	// The actual query is built by buildAttemptsQuery, which is complex.
	// For unit testing the adapter, we focus on the DB interaction part.
	// We assume buildAttemptsQuery is correct or tested separately if complex enough.
	// This regex is for the "SELECT uqa.* FROM user_quiz_attempts uqa WHERE uqa.user_id = :user_id AND uqa.deleted_at IS NULL ORDER BY ... " part
	mock.ExpectPrepare(regexp.QuoteMeta("SELECT uqa.* FROM user_quiz_attempts uqa WHERE uqa.user_id = :user_id AND uqa.deleted_at IS NULL ORDER BY uqa.attempted_at DESC OFFSET :offset ROWS FETCH NEXT :limit ROWS ONLY")).
		ExpectQuery().
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), userID). // limit, offset, user_id
		WillReturnRows(rows)

	// Mock for the count query
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(len(expectedModels))
	mock.ExpectPrepare(regexp.QuoteMeta("SELECT COUNT(*) FROM user_quiz_attempts uqa WHERE uqa.user_id = :user_id AND uqa.deleted_at IS NULL")).
		ExpectQuery().
		WithArgs(userID). // user_id
		WillReturnRows(countRows)

	filters := dto.AttemptFilters{}
	pagination := dto.Pagination{Limit: 10, Offset: 0}
	domainAttempts, total, err := repo.GetAttemptsByUserID(context.Background(), userID, filters, pagination)

	assert.NoError(t, err)
	assert.NotNil(t, domainAttempts)
	assert.Len(t, domainAttempts, len(expectedModels))
	assert.Equal(t, len(expectedModels), total)
	if len(domainAttempts) > 0 {
		assert.Equal(t, expectedModels[0].ID, domainAttempts[0].ID)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
