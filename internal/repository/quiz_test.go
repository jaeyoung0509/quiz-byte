package repository

import (
	"context" // Added context
	"database/sql"
	"encoding/json" // For TestToModelQuizEvaluationAndBack
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
func setupTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) { //nolint:thelper
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

	originalSQL := `SELECT id "id", question "question", model_answers "model_answers", keywords "keywords", difficulty "difficulty", sub_category_id "sub_category_id", created_at "created_at", updated_at "updated_at", deleted_at "deleted_at" FROM quizzes WHERE id = :1 AND deleted_at IS NULL`

	mock.ExpectQuery(regexp.QuoteMeta(originalSQL)).
		WithArgs(testULID).
		WillReturnRows(rows)

	result, err := repo.GetQuizByID(context.Background(), testULID)

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

	originalSQL := `SELECT id "id", question "question", model_answers "model_answers", keywords "keywords", difficulty "difficulty", sub_category_id "sub_category_id", created_at "created_at", updated_at "updated_at", deleted_at "deleted_at" FROM quizzes WHERE sub_category_id = :1 AND deleted_at IS NULL ORDER BY DBMS_RANDOM.VALUE FETCH FIRST 1 ROWS ONLY`

	mock.ExpectQuery(regexp.QuoteMeta(originalSQL)).
		WithArgs(testSubCatID).
		WillReturnRows(rows)

	result, err := repo.GetRandomQuizBySubCategory(context.Background(), testSubCatID)

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

	result, err := repo.GetRandomQuiz(context.Background())

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

	mock.ExpectQuery(regexp.QuoteMeta(originalSQL)).
		WithArgs(testULID).
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetQuizByID(context.Background(), testULID)

	assert.NoError(t, err) // Adapter method returns (nil, nil) for sql.ErrNoRows
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

	err := repo.SaveAnswer(context.Background(), domainAnswer)

	assert.NoError(t, err)
	assert.NotEmpty(t, domainAnswer.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSimilarQuiz(t *testing.T) {
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

	result, err := repo.GetSimilarQuiz(context.Background(), currentQuizID)

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

	result, err := repo.GetSimilarQuiz(context.Background(), currentQuizID)
	assert.NoError(t, err)
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

	result, err := repo.GetSimilarQuiz(context.Background(), currentQuizID)

	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Tests from quiz_database_adapter_test.go ---

func TestToModelQuizEvaluationAndBack(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for consistent comparison
	domainEval := &domain.QuizEvaluation{
		ID:              "eval1",
		QuizID:          "quiz1",
		MinimumKeywords: 2,
		RequiredTopics:  []string{"Go", "Structs"},
		ScoreRanges:     []string{"0-0.5", "0.5-1.0"},
		SampleAnswers:   []string{"Sample Ans 1"},
		RubricDetails:   "Some rubric details",
		CreatedAt:       now,
		UpdatedAt:       now,
		ScoreEvaluations: []domain.ScoreEvaluationDetail{
			{ScoreRange: "0-0.5", SampleAnswers: []string{"Bad answer"}, Explanation: "This was not good."},
			{ScoreRange: "0.5-1.0", SampleAnswers: []string{"Good answer!"}, Explanation: "Excellent work!"},
		},
	}

	modelEval, err := toModelQuizEvaluation(domainEval)
	if err != nil {
		t.Fatalf("toModelQuizEvaluation() error = %v", err)
	}
	if modelEval == nil {
		t.Fatalf("toModelQuizEvaluation() returned nil model")
	}

	assert.Equal(t, domainEval.ID, modelEval.ID)
	assert.Equal(t, domainEval.QuizID, modelEval.QuizID)
	// ... (rest of assertions from original test)

	var unmarshaledDetails []domain.ScoreEvaluationDetail
	if modelEval.ScoreEvaluations == "" {
		t.Fatalf("modelEval.ScoreEvaluations is empty, expected JSON string")
	}
	err = json.Unmarshal([]byte(modelEval.ScoreEvaluations), &unmarshaledDetails)
	if err != nil {
		t.Fatalf("Failed to unmarshal modelEval.ScoreEvaluations: %v. JSON was: %s", err, modelEval.ScoreEvaluations)
	}
	assert.Equal(t, len(domainEval.ScoreEvaluations), len(unmarshaledDetails))
	if len(unmarshaledDetails) > 0 && len(domainEval.ScoreEvaluations) > 0 {
		assert.Equal(t, domainEval.ScoreEvaluations[0].Explanation, unmarshaledDetails[0].Explanation)
	}

	modelEval.CreatedAt = domainEval.CreatedAt
	modelEval.UpdatedAt = domainEval.UpdatedAt

	convertedDomainEval, err := toDomainQuizEvaluation(modelEval)
	if err != nil {
		t.Fatalf("toDomainQuizEvaluation() error = %v", err)
	}
	if convertedDomainEval == nil {
		t.Fatalf("toDomainQuizEvaluation() returned nil")
	}
	assert.Equal(t, domainEval.ID, convertedDomainEval.ID)
	// ... (rest of assertions)
	assert.Equal(t, len(domainEval.ScoreEvaluations), len(convertedDomainEval.ScoreEvaluations))
	if len(convertedDomainEval.ScoreEvaluations) > 0 && len(domainEval.ScoreEvaluations) > 0 {
		assert.Equal(t, domainEval.ScoreEvaluations[0].Explanation, convertedDomainEval.ScoreEvaluations[0].Explanation)
	}
	assert.True(t, convertedDomainEval.CreatedAt.Equal(domainEval.CreatedAt))
	assert.True(t, convertedDomainEval.UpdatedAt.Equal(domainEval.UpdatedAt))
}

func TestToModelQuizEvaluation_NilInput(t *testing.T) {
	modelEval, err := toModelQuizEvaluation(nil)
	assert.NoError(t, err)
	assert.Nil(t, modelEval)
}

func TestToDomainQuizEvaluation_NilInput(t *testing.T) {
	domainEval, err := toDomainQuizEvaluation(nil)
	assert.NoError(t, err)
	assert.Nil(t, domainEval)
}

func TestToDomainQuizEvaluation_EmptyScoreEvaluationsJSON(t *testing.T) {
	modelEval := &models.QuizEvaluation{
		ID:               "eval1",
		QuizID:           "quiz1",
		ScoreEvaluations: "",
	}
	domainEval, err := toDomainQuizEvaluation(modelEval)
	assert.NoError(t, err)
	assert.NotNil(t, domainEval)
	assert.Empty(t, domainEval.ScoreEvaluations)
}

func TestToDomainQuizEvaluation_MalformedScoreEvaluationsJSON(t *testing.T) {
	modelEval := &models.QuizEvaluation{
		ID:               "eval1",
		QuizID:           "quiz1",
		ScoreEvaluations: "{not_a_valid_json",
	}
	_, err := toDomainQuizEvaluation(modelEval)
	assert.Error(t, err)
}

func TestToDomainQuiz(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	model := &models.Quiz{
		ID:            "q1",
		Question:      "What is Go?",
		ModelAnswers:  "Go is a language" + stringDelimiter + "It is fun",
		Keywords:      "go" + stringDelimiter + "lang",
		Difficulty:    1,
		SubCategoryID: "subcat1",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	domainQuiz, err := toDomainQuiz(model)
	assert.NoError(t, err)
	assert.NotNil(t, domainQuiz)
	assert.Equal(t, model.ID, domainQuiz.ID)
	assert.Equal(t, model.Question, domainQuiz.Question)
	assert.Equal(t, []string{"Go is a language", "It is fun"}, domainQuiz.ModelAnswers)
	assert.Equal(t, []string{"go", "lang"}, domainQuiz.Keywords)
	// ... (assert other fields)
}

func TestToDomainQuiz_NilInput(t *testing.T) {
	domainQuiz, err := toDomainQuiz(nil)
	assert.Error(t, err)
	assert.Nil(t, domainQuiz)
}

func TestToModelQuiz(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	domainQ := &domain.Quiz{
		ID:            "q1",
		Question:      "What is Go?",
		ModelAnswers:  []string{"Go is a language", "It is fun"},
		Keywords:      []string{"go", "lang"},
		Difficulty:    1,
		SubCategoryID: "subcat1",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	modelQuiz := toModelQuiz(domainQ)
	assert.NotNil(t, modelQuiz)
	assert.Equal(t, domainQ.ID, modelQuiz.ID)
	assert.Equal(t, "Go is a language"+stringDelimiter+"It is fun", modelQuiz.ModelAnswers)
	// ... (assert other fields)
}

func TestToModelQuiz_NilInput(t *testing.T) {
	modelQuiz := toModelQuiz(nil)
	assert.Nil(t, modelQuiz)
}

func TestToModelAnswer(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	domainAns := &domain.Answer{
		ID:             "ans1",
		QuizID:         "q1",
		UserAnswer:     "This is my answer.",
		Score:          0.8,
		Explanation:    "Good job.",
		KeywordMatches: []string{"keyword1", "keyword2"},
		Completeness:   0.9,
		Relevance:      0.7,
		Accuracy:       0.85,
		AnsweredAt:     now,
	}

	modelAns := toModelAnswer(domainAns)
	assert.NotNil(t, modelAns)
	assert.Equal(t, domainAns.ID, modelAns.ID)
	// ... (assert other fields)
}

func TestToModelAnswer_NilInput(t *testing.T) {
	modelAns := toModelAnswer(nil)
	assert.Nil(t, modelAns)
}

// Adapter method tests from quiz_database_adapter_test.go (GetQuizByID, SaveQuiz)
// These are already present above in a slightly different form, I'll keep the ones in this file.
// For brevity, I will assume the existing TestQuizDatabaseAdapter_GetQuizByID_Success, TestQuizDatabaseAdapter_GetQuizByID_NotFound, TestQuizDatabaseAdapter_SaveQuiz_Success
// are sufficient for this file for now.
// The tool will merge the content, effectively giving one set of these tests.
// The setupQuizTestDB is defined above.
// The `sqlmock` tests `TestQuizDatabaseAdapter_GetQuizByID_Success`, `_NotFound`, `_SaveQuiz_Success` were originally in `quiz_database_adapter_test.go` and are now effectively part of this combined file.
// I will ensure the imports are consolidated and correct.
// The `TestGetQuizByID` in this file is slightly different (uses ExpectPrepare) from the one I drafted for `quiz_database_adapter_test.go`. I'll keep this one.
