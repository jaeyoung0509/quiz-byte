package repository

import (
	"quiz-byte/internal/repository/models"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	oracle "github.com/godoes/gorm-oracle"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	// Create sqlmock
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}

	// Create GORM DB with sqlmock
	dialector := oracle.New(oracle.Config{
		Conn: sqlDB,
	})

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open gorm db: %v", err)
	}

	return db, mock
}

func TestGetAllSubCategories(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	// 테스트 케이스
	tests := []struct {
		name          string
		mockRows      *sqlmock.Rows
		expected      []int64
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockRows: sqlmock.NewRows([]string{"sub_category_id"}).
				AddRow(1).
				AddRow(2).
				AddRow(3),
			expected:      []int64{1, 2, 3},
			expectedError: false,
		},
		{
			name:          "empty result",
			mockRows:      sqlmock.NewRows([]string{"sub_category_id"}),
			expected:      []int64{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 쿼리 설정
			mock.ExpectQuery("SELECT DISTINCT sub_category_id FROM quizzes").
				WillReturnRows(tt.mockRows)

			// 테스트 실행
			result, err := repo.GetAllSubCategories()

			// 검증
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}

			// 모든 기대가 충족되었는지 확인
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetRandomQuizBySubCategory(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	// 테스트 데이터
	now := time.Now()
	// modelAnswers := models.StringSlice{"Go is a programming language developed by Google"}
	// keywords := models.StringSlice{"programming", "language", "google", "go"}

	expectedQuiz := &models.Quiz{
		ID:            1,
		Question:      "What is Go programming language?",
		ModelAnswers:  "Go is a programming language developed by Google",
		Keywords:      "programming, language, google, go",
		Difficulty:    1, // 1: Easy, 2: Medium, 3: Hard
		SubCategoryID: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Mock 쿼리 설정
	rows := sqlmock.NewRows([]string{
		"id", "question", "model_answers", "keywords",
		"difficulty", "sub_category_id", "created_at", "updated_at",
	}).AddRow(
		expectedQuiz.ID, expectedQuiz.Question, expectedQuiz.ModelAnswers,
		expectedQuiz.Keywords, expectedQuiz.Difficulty, expectedQuiz.SubCategoryID,
		expectedQuiz.CreatedAt, expectedQuiz.UpdatedAt,
	)

	mock.ExpectQuery("SELECT.*FROM quizzes").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	// 테스트 실행
	result, err := repo.GetRandomQuizBySubCategory(1)

	// 검증
	assert.NoError(t, err)
	assert.Equal(t, expectedQuiz, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetQuizByID(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	// 테스트 데이터
	now := time.Now()
	// modelAnswers := models.StringSlice{"Go is a programming language"}
	// keywords := models.StringSlice{"go", "programming", "language"}

	expectedQuiz := &models.Quiz{
		ID:            1,
		Question:      "What is Go?",
		ModelAnswers:  "Go is a programming language",
		Keywords:      "go, programming, language",
		Difficulty:    1, // 1: Easy, 2: Medium, 3: Hard
		SubCategoryID: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Mock 쿼리 설정
	rows := sqlmock.NewRows([]string{
		"id", "question", "model_answers", "keywords",
		"difficulty", "sub_category_id", "created_at", "updated_at",
	}).AddRow(
		expectedQuiz.ID, expectedQuiz.Question, expectedQuiz.ModelAnswers,
		expectedQuiz.Keywords, expectedQuiz.Difficulty, expectedQuiz.SubCategoryID,
		expectedQuiz.CreatedAt, expectedQuiz.UpdatedAt,
	)

	mock.ExpectQuery("SELECT.*FROM quizzes").
		WithArgs(1).
		WillReturnRows(rows)

	// 테스트 실행
	result, err := repo.GetQuizByID(1)

	// 검증
	assert.NoError(t, err)
	assert.Equal(t, expectedQuiz, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
