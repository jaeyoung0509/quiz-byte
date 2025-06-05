package repository

import (
	"context" // Added context
	"database/sql"
	"regexp"
	"testing"
	"time"

	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"

	"strings" // Added for strings.Join

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// setupTestDB creates a new sqlx.DB instance and sqlmock for testing.
func setupTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return sqlxDB, mock
}

func TestGetQuizByID(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	testULID := util.NewULID()
	now := time.Now()

	expectedModelQuiz := models.Quiz{
		ID:            testULID,
		Question:      "What is Go?",
		ModelAnswers:  "Go is a programming language" + stringDelimiter + "Developed by Google",
		Keywords:      "go" + stringDelimiter + "programming" + stringDelimiter + "language",
		Difficulty:    1,
		SubCategoryID: util.NewULID(),
		CreatedAt:     now,
		UpdatedAt:     now,
		DeletedAt:     nil,
	}

	rows := sqlmock.NewRows([]string{"id", "question", "model_answers", "keywords", "difficulty", "sub_category_id", "created_at", "updated_at", "deleted_at"}).
		AddRow(expectedModelQuiz.ID, expectedModelQuiz.Question, expectedModelQuiz.ModelAnswers, expectedModelQuiz.Keywords, expectedModelQuiz.Difficulty, expectedModelQuiz.SubCategoryID, expectedModelQuiz.CreatedAt, expectedModelQuiz.UpdatedAt, expectedModelQuiz.DeletedAt)

	// sqlx translates :named parameters to ? for many drivers before preparing.
	// For sqlmock, we need to match the query style used by the driver, which might be '?' after sqlx rebinding.
	// However, the Prepare statement in the code itself uses the original query with named args.
	// The error message shows the "actual sql" is the one with ":1".
	// So, we should match that in ExpectPrepare.
	originalSQL := `SELECT id "id", question "question", model_answers "model_answers", keywords "keywords", difficulty "difficulty", sub_category_id "sub_category_id", created_at "created_at", updated_at "updated_at", deleted_at "deleted_at" FROM quizzes WHERE id = :1 AND deleted_at IS NULL`

	mock.ExpectPrepare(regexp.QuoteMeta(originalSQL)).
		ExpectQuery().
		WithArgs(testULID).
		WillReturnRows(rows)

	result, err := repo.GetQuizByID(testULID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedModelQuiz.ID, result.ID)
	assert.Equal(t, expectedModelQuiz.Question, result.Question)
	assert.Equal(t, []string{"Go is a programming language", "Developed by Google"}, result.ModelAnswers)
	assert.Equal(t, []string{"go", "programming", "language"}, result.Keywords)
	assert.Equal(t, expectedModelQuiz.Difficulty, result.Difficulty)
	assert.Equal(t, expectedModelQuiz.SubCategoryID, result.SubCategoryID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllSubCategories(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	tests := []struct {
		name          string
		mockRows      *sqlmock.Rows
		expected      []string
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockRows: sqlmock.NewRows([]string{"sub_category_id"}).
				AddRow("cat1").
				AddRow("cat2").
				AddRow("cat3"),
			expected:      []string{"cat1", "cat2", "cat3"},
			expectedError: false,
		},
		{
			name:          "empty result",
			mockRows:      sqlmock.NewRows([]string{"sub_category_id"}),
			expected:      []string{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedQuery := `SELECT DISTINCT sub_category_id FROM quizzes WHERE sub_category_id IS NOT NULL AND deleted_at IS NULL ORDER BY sub_category_id ASC`
			mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
				WillReturnRows(tt.mockRows)

			result, err := repo.GetAllSubCategories(context.Background()) // Added context

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetRandomQuizBySubCategory(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	now := time.Now()
	testSubCatID := util.NewULID()
	expectedModelQuiz := models.Quiz{
		ID:            util.NewULID(),
		Question:      "What is Go programming language in subcat?",
		ModelAnswers:  "Go is a programming language developed by Google",
		Keywords:      "programming,language,google,go",
		Difficulty:    1,
		SubCategoryID: testSubCatID,
		CreatedAt:     now,
		UpdatedAt:     now,
		DeletedAt:     nil,
	}

	rows := sqlmock.NewRows([]string{"id", "question", "model_answers", "keywords", "difficulty", "sub_category_id", "created_at", "updated_at", "deleted_at"}).
		AddRow(expectedModelQuiz.ID, expectedModelQuiz.Question, expectedModelQuiz.ModelAnswers, expectedModelQuiz.Keywords, expectedModelQuiz.Difficulty, expectedModelQuiz.SubCategoryID, expectedModelQuiz.CreatedAt, expectedModelQuiz.UpdatedAt, expectedModelQuiz.DeletedAt)

	// sqlx translates :named parameters to ? for many drivers before preparing.
	originalSQL := `SELECT id "id", question "question", model_answers "model_answers", keywords "keywords", difficulty "difficulty", sub_category_id "sub_category_id", created_at "created_at", updated_at "updated_at", deleted_at "deleted_at" FROM quizzes WHERE sub_category_id = :1 AND deleted_at IS NULL ORDER BY DBMS_RANDOM.VALUE FETCH FIRST 1 ROWS ONLY`

	mock.ExpectPrepare(regexp.QuoteMeta(originalSQL)).
		ExpectQuery().
		WithArgs(testSubCatID).
		WillReturnRows(rows)

	result, err := repo.GetRandomQuizBySubCategory(testSubCatID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedModelQuiz.ID, result.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRandomQuiz(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	now := time.Now()
	expectedModelQuiz := models.Quiz{
		ID:            util.NewULID(),
		Question:      "What is a random Go fact?",
		ModelAnswers:  "Go has a mascot, the Gopher." + stringDelimiter + "It's an open-source project.",
		Keywords:      "go" + stringDelimiter + "gopher" + stringDelimiter + "random",
		Difficulty:    2,
		SubCategoryID: util.NewULID(),
		CreatedAt:     now,
		UpdatedAt:     now,
		DeletedAt:     nil,
	}

	rows := sqlmock.NewRows([]string{"id", "question", "model_answers", "keywords", "difficulty", "sub_category_id", "created_at", "updated_at", "deleted_at"}).
		AddRow(expectedModelQuiz.ID, expectedModelQuiz.Question, expectedModelQuiz.ModelAnswers, expectedModelQuiz.Keywords, expectedModelQuiz.Difficulty, expectedModelQuiz.SubCategoryID, expectedModelQuiz.CreatedAt, expectedModelQuiz.UpdatedAt, expectedModelQuiz.DeletedAt)

	originalSQL := `SELECT
		id "id",
		question "question",
		model_answers "model_answers",
		keywords "keywords",
		difficulty "difficulty",
		sub_category_id "sub_category_id",
		created_at "created_at",
		updated_at "updated_at",
		deleted_at "deleted_at"
	FROM quizzes
	WHERE deleted_at IS NULL
	ORDER BY DBMS_RANDOM.VALUE
	FETCH FIRST 1 ROWS ONLY`

	mock.ExpectQuery(regexp.QuoteMeta(originalSQL)).
		WillReturnRows(rows)

	result, err := repo.GetRandomQuiz()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedModelQuiz.ID, result.ID)
	assert.Equal(t, expectedModelQuiz.Question, result.Question)
	assert.Equal(t, []string{"Go has a mascot, the Gopher.", "It's an open-source project."}, result.ModelAnswers)
	assert.Equal(t, []string{"go", "gopher", "random"}, result.Keywords)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetQuizByID_NotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)
	testULID := util.NewULID()

	originalSQL := `SELECT id "id", question "question", model_answers "model_answers", keywords "keywords", difficulty "difficulty", sub_category_id "sub_category_id", created_at "created_at", updated_at "updated_at", deleted_at "deleted_at" FROM quizzes WHERE id = :1 AND deleted_at IS NULL`

	mock.ExpectPrepare(regexp.QuoteMeta(originalSQL)).
		ExpectQuery().
		WithArgs(testULID).
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetQuizByID(testULID)

	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveAnswer(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	quizID := util.NewULID()
	domainAnswer := &domain.Answer{
		QuizID:         quizID,
		UserAnswer:     "This is my answer.",
		Score:          0.85,
		Explanation:    "Good answer.",
		KeywordMatches: []string{"good", "answer"},
		Completeness:   0.9,
		Relevance:      0.9,
		Accuracy:       0.8,
	}

	originalSQL := `INSERT INTO answers (
        id, quiz_id, user_answer, score, explanation,
        keyword_matches, completeness, relevance, accuracy,
        answered_at, created_at, updated_at
    ) VALUES (
        :1, :2, :3, :4, :5, :6, :7, :8, :9, :10, :11, :12
    )`

	mock.ExpectExec(regexp.QuoteMeta(originalSQL)).
		WithArgs(
			sqlmock.AnyArg(), // id
			domainAnswer.QuizID,
			domainAnswer.UserAnswer,
			domainAnswer.Score,
			domainAnswer.Explanation,
			strings.Join(domainAnswer.KeywordMatches, stringDelimiter),
			domainAnswer.Completeness,
			domainAnswer.Relevance,
			domainAnswer.Accuracy,
			sqlmock.AnyArg(), // answered_at
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.SaveAnswer(domainAnswer)

	assert.NoError(t, err)
	assert.NotEmpty(t, domainAnswer.ID)
	// assert.NotZero(t, domainAnswer.AnsweredAt) // AnsweredAt is set by the caller, not SaveAnswer
	// CreatedAt and UpdatedAt are not fields on domain.Answer, they are set in model before saving
	// So we cannot assert them on domainAnswer directly after save.
	// If these were on domain.Answer, we would assert them:
	// assert.NotZero(t, domainAnswer.CreatedAt)
	// assert.NotZero(t, domainAnswer.UpdatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSimilarQuiz(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	currentQuizID := util.NewULID()
	subCatID := util.NewULID()
	difficulty := 2

	// Mock for the first query (getting current quiz details)
	originalQueryCurrent := `SELECT
		difficulty "difficulty",
		sub_category_id "sub_category_id"
	FROM quizzes
	WHERE id = :1
	AND deleted_at IS NULL`
	mock.ExpectPrepare(regexp.QuoteMeta(originalQueryCurrent)).
		ExpectQuery().
		WithArgs(currentQuizID).
		WillReturnRows(sqlmock.NewRows([]string{"difficulty", "sub_category_id"}).AddRow(difficulty, subCatID))

	// Mock for the second query (getting similar quiz)
	similarQuizID := util.NewULID()
	now := time.Now()
	expectedSimilarModelQuiz := models.Quiz{
		ID:            similarQuizID,
		Question:      "A similar question?",
		ModelAnswers:  "Similar answer",
		Keywords:      "similar,keyword",
		Difficulty:    difficulty,
		SubCategoryID: subCatID,
		CreatedAt:     now,
		UpdatedAt:     now,
		DeletedAt:     nil,
	}
	rowsSimilar := sqlmock.NewRows([]string{"id", "question", "model_answers", "keywords", "difficulty", "sub_category_id", "created_at", "updated_at", "deleted_at"}).
		AddRow(expectedSimilarModelQuiz.ID, expectedSimilarModelQuiz.Question, expectedSimilarModelQuiz.ModelAnswers, expectedSimilarModelQuiz.Keywords, expectedSimilarModelQuiz.Difficulty, expectedSimilarModelQuiz.SubCategoryID, expectedSimilarModelQuiz.CreatedAt, expectedSimilarModelQuiz.UpdatedAt, expectedSimilarModelQuiz.DeletedAt)

	originalQuerySimilar := `SELECT
		id "id",
		question "question",
		model_answers "model_answers",
		keywords "keywords",
		difficulty "difficulty",
		sub_category_id "sub_category_id",
		created_at "created_at",
		updated_at "updated_at",
		deleted_at "deleted_at"
	FROM quizzes
	WHERE id != :1
	AND sub_category_id = :2
	AND difficulty = :3
	AND deleted_at IS NULL
	ORDER BY DBMS_RANDOM.VALUE
	FETCH FIRST 1 ROWS ONLY`
	mock.ExpectPrepare(regexp.QuoteMeta(originalQuerySimilar)).
		ExpectQuery().
		WithArgs(currentQuizID, subCatID, difficulty).
		WillReturnRows(rowsSimilar)

	result, err := repo.GetSimilarQuiz(currentQuizID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, similarQuizID, result.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSimilarQuiz_CurrentQuizNotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)
	currentQuizID := util.NewULID()

	originalQueryCurrent := `SELECT
		difficulty "difficulty",
		sub_category_id "sub_category_id"
	FROM quizzes
	WHERE id = :1
	AND deleted_at IS NULL`
	mock.ExpectPrepare(regexp.QuoteMeta(originalQueryCurrent)).
		ExpectQuery().
		WithArgs(currentQuizID).
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetSimilarQuiz(currentQuizID)
	assert.NoError(t, err) // Adapter transforms sql.ErrNoRows to (nil,nil)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSimilarQuiz_SimilarQuizNotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	repo := NewQuizDatabaseAdapter(db)

	currentQuizID := util.NewULID()
	subCatID := util.NewULID()
	difficulty := 2

	originalQueryCurrent := `SELECT
		difficulty "difficulty",
		sub_category_id "sub_category_id"
	FROM quizzes
	WHERE id = :1
	AND deleted_at IS NULL`
	mock.ExpectPrepare(regexp.QuoteMeta(originalQueryCurrent)).
		ExpectQuery().
		WithArgs(currentQuizID).
		WillReturnRows(sqlmock.NewRows([]string{"difficulty", "sub_category_id"}).AddRow(difficulty, subCatID))

	originalQuerySimilar := `SELECT
		id "id",
		question "question",
		model_answers "model_answers",
		keywords "keywords",
		difficulty "difficulty",
		sub_category_id "sub_category_id",
		created_at "created_at",
		updated_at "updated_at",
		deleted_at "deleted_at"
	FROM quizzes
	WHERE id != :1
	AND sub_category_id = :2
	AND difficulty = :3
	AND deleted_at IS NULL
	ORDER BY DBMS_RANDOM.VALUE
	FETCH FIRST 1 ROWS ONLY`
	mock.ExpectPrepare(regexp.QuoteMeta(originalQuerySimilar)).
		ExpectQuery().
		WithArgs(currentQuizID, subCatID, difficulty).
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetSimilarQuiz(currentQuizID)

	assert.NoError(t, err) // Adapter transforms sql.ErrNoRows to (nil,nil)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
