package repository

import (
	"context"
	"database/sql"
	"encoding/json" // For JSON marshaling/unmarshaling
	"fmt"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto" // Added for dto.QuizRecommendationItem
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"strings"
	"time"
)

const stringDelimiter = "|||"

// QuizDatabaseAdapter implements domain.QuizRepository using sqlx.DB
type QuizDatabaseAdapter struct {
	db DBTX
}

// NewQuizDatabaseAdapter creates a new instance of QuizDatabaseAdapter
func NewQuizDatabaseAdapter(db DBTX) domain.QuizRepository {
	return &QuizDatabaseAdapter{db: db}
}

// GetRandomQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuiz(ctx context.Context) (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	query := `SELECT 
		id "ID",
		question "QUESTION",
		model_answers "MODEL_ANSWERS",
		keywords "KEYWORDS",
		difficulty "DIFFICULTY",
		sub_category_id "SUB_CATEGORY_ID",
		created_at "CREATED_AT",
		updated_at "UPDATED_AT",
		deleted_at "DELETED_AT"
	FROM quizzes 
	WHERE deleted_at IS NULL 
	ORDER BY DBMS_RANDOM.VALUE 
	FETCH FIRST 1 ROWS ONLY`

	err := a.db.GetContext(ctx, &modelQuiz, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get random quiz: %w", err)
	}
	return toDomainQuiz(&modelQuiz)
}

// GetQuizByID implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizByID(ctx context.Context, id string) (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	query := `SELECT 
		id "ID",
		question "QUESTION",
		model_answers "MODEL_ANSWERS",
		keywords "KEYWORDS",
		difficulty "DIFFICULTY",
		sub_category_id "SUB_CATEGORY_ID",
		created_at "CREATED_AT",
		updated_at "UPDATED_AT",
		deleted_at "DELETED_AT"
	FROM quizzes 
	WHERE id = :1 
	AND deleted_at IS NULL`

	err := a.db.GetContext(ctx, &modelQuiz, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get quiz by ID %s: %w", id, err)
	}
	return toDomainQuiz(&modelQuiz)
}

// SaveQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveQuiz(ctx context.Context, quiz *domain.Quiz) error {
	modelQuiz := toModelQuiz(quiz)
	if modelQuiz == nil {
		return fmt.Errorf("cannot save nil quiz")
	}
	modelQuiz.ID = util.NewULID()
	modelQuiz.CreatedAt = time.Now()
	modelQuiz.UpdatedAt = time.Now()

	query := `INSERT INTO quizzes (
		id, question, model_answers, keywords, 
		difficulty, sub_category_id, created_at, updated_at
	) VALUES (
		:1, :2, :3, :4, :5, :6, :7, :8
	)`

	executor := GetExecutor(ctx, a.db)
	_, err := executor.ExecContext(ctx, query,
		modelQuiz.ID,
		modelQuiz.Question,
		modelQuiz.ModelAnswers,
		modelQuiz.Keywords,
		modelQuiz.Difficulty,
		modelQuiz.SubCategoryID,
		modelQuiz.CreatedAt,
		modelQuiz.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save quiz: %w", err)
	}

	quiz.ID = modelQuiz.ID
	quiz.CreatedAt = modelQuiz.CreatedAt
	quiz.UpdatedAt = modelQuiz.UpdatedAt
	return nil
}

// UpdateQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) UpdateQuiz(ctx context.Context, quiz *domain.Quiz) error {
	modelQuiz := toModelQuiz(quiz)
	if modelQuiz == nil {
		return fmt.Errorf("cannot update nil quiz")
	}
	if modelQuiz.ID == "" { // ULIDs are strings
		return fmt.Errorf("cannot update quiz with empty ID")
	}
	modelQuiz.UpdatedAt = time.Now()

	query := `UPDATE quizzes SET 
		question = :1, 
		model_answers = :2, 
		keywords = :3, 
		difficulty = :4, 
		sub_category_id = :5, 
		updated_at = :6
	WHERE id = :7 
	AND deleted_at IS NULL`

	result, err := a.db.ExecContext(ctx, query,
		modelQuiz.Question,
		modelQuiz.ModelAnswers,
		modelQuiz.Keywords,
		modelQuiz.Difficulty,
		modelQuiz.SubCategoryID,
		modelQuiz.UpdatedAt,
		modelQuiz.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update quiz: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("quiz with ID %s not found or not updated", quiz.ID)
	}
	quiz.UpdatedAt = modelQuiz.UpdatedAt
	return nil
}

// SaveAnswer implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveAnswer(ctx context.Context, answer *domain.Answer) error {
	modelAnswer := toModelAnswer(answer)
	if modelAnswer == nil {
		return fmt.Errorf("cannot save nil answer")
	}
	modelAnswer.ID = util.NewULID()
	modelAnswer.CreatedAt = time.Now()
	modelAnswer.UpdatedAt = time.Now()

	query := `INSERT INTO answers (
        id, quiz_id, user_answer, score, explanation, 
        keyword_matches, completeness, relevance, accuracy, 
        answered_at, created_at, updated_at
    ) VALUES (
        :1, :2, :3, :4, :5, :6, :7, :8, :9, :10, :11, :12
    )`

	_, err := a.db.ExecContext(ctx, query,
		modelAnswer.ID,
		modelAnswer.QuizID,
		modelAnswer.UserAnswer,
		modelAnswer.Score,
		modelAnswer.Explanation,
		strings.Join(modelAnswer.KeywordMatches, stringDelimiter),
		modelAnswer.Completeness,
		modelAnswer.Relevance,
		modelAnswer.Accuracy,
		modelAnswer.AnsweredAt,
		modelAnswer.CreatedAt,
		modelAnswer.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save answer: %w", err)
	}

	answer.ID = modelAnswer.ID
	return nil
}

// GetSimilarQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetSimilarQuiz(ctx context.Context, quizID string) (*domain.Quiz, error) {
	current := struct {
		Difficulty    int    `db:"DIFFICULTY"`
		SubCategoryID string `db:"SUB_CATEGORY_ID"`
	}{}
	queryCurrent := `SELECT 
		difficulty "DIFFICULTY", 
		sub_category_id "SUB_CATEGORY_ID" 
	FROM quizzes 
	WHERE id = :1 
	AND deleted_at IS NULL`

	err := a.db.GetContext(ctx, &current, queryCurrent, quizID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Current quiz not found
		}
		return nil, fmt.Errorf("failed to get current quiz for similarity check: %w", err)
	}

	var similarQuizModel models.Quiz
	querySimilar := `SELECT 
		id "ID",
		question "QUESTION",
		model_answers "MODEL_ANSWERS",
		keywords "KEYWORDS",
		difficulty "DIFFICULTY",
		sub_category_id "SUB_CATEGORY_ID",
		created_at "CREATED_AT",
		updated_at "UPDATED_AT",
		deleted_at "DELETED_AT"
	FROM quizzes
	WHERE id != :1 
	AND sub_category_id = :2 
	AND difficulty = :3 
	AND deleted_at IS NULL
	ORDER BY DBMS_RANDOM.VALUE 
	FETCH FIRST 1 ROWS ONLY`

	err = a.db.GetContext(ctx, &similarQuizModel, querySimilar, quizID, current.SubCategoryID, current.Difficulty)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No similar quiz found
		}
		return nil, fmt.Errorf("failed to get similar quiz: %w", err)
	}
	return toDomainQuiz(&similarQuizModel)
}

// GetAllSubCategories implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetAllSubCategories(ctx context.Context) ([]string, error) {
	var subCategoryIDs []string
	query := `SELECT id FROM sub_categories WHERE deleted_at IS NULL ORDER BY id ASC`
	err := a.db.SelectContext(ctx, &subCategoryIDs, query) // Already using SelectContext
	if err != nil {
		return nil, fmt.Errorf("failed to query sub categories: %w", err)
	}

	if subCategoryIDs == nil { // Should not happen with SelectContext but good for safety
		return []string{}, nil
	}
	return subCategoryIDs, nil
}

// SaveQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveQuizEvaluation(ctx context.Context, evaluation *domain.QuizEvaluation) error {
	if evaluation.ID == "" {
		evaluation.ID = util.NewULID()
	}
	now := time.Now()
	evaluation.CreatedAt = now
	evaluation.UpdatedAt = now

	modelEval, err := toModelQuizEvaluation(evaluation)
	if err != nil {
		return fmt.Errorf("failed to convert domain.QuizEvaluation to model: %w", err)
	}
	if modelEval == nil {
		return fmt.Errorf("cannot save nil QuizEvaluation")
	}

	modelEval.CreatedAt = evaluation.CreatedAt
	modelEval.UpdatedAt = evaluation.UpdatedAt

	query := `INSERT INTO quiz_evaluations (
		id, quiz_id, minimum_keywords, required_topics, score_ranges,
		sample_answers, rubric_details, created_at, updated_at, score_evaluations
	) VALUES (
		:1, :2, :3, :4, :5,
		:6, :7, :8, :9, :10
	)`

	executor := GetExecutor(ctx, a.db)
	_, err = executor.ExecContext(ctx, query,
		modelEval.ID, modelEval.QuizID, modelEval.MinimumKeywords,
		modelEval.RequiredTopics, modelEval.ScoreRanges,
		modelEval.SampleAnswers, modelEval.RubricDetails,
		modelEval.CreatedAt, modelEval.UpdatedAt, modelEval.ScoreEvaluations)
	if err != nil {
		return fmt.Errorf("failed to save quiz evaluation for quiz_id %s: %w", modelEval.QuizID, err)
	}

	return nil
}

// GetQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizEvaluation(ctx context.Context, quizID string) (*domain.QuizEvaluation, error) {
	var modelEval models.QuizEvaluation
	query := `SELECT
		id "ID", quiz_id "QUIZ_ID", minimum_keywords "MINIMUM_KEYWORDS", 
		required_topics "REQUIRED_TOPICS", score_ranges "SCORE_RANGES",
		sample_answers "SAMPLE_ANSWERS", rubric_details "RUBRIC_DETAILS", 
		created_at "CREATED_AT", updated_at "UPDATED_AT", deleted_at "DELETED_AT", 
		score_evaluations "SCORE_EVALUATIONS"
	FROM quiz_evaluations
	WHERE quiz_id = :1 AND deleted_at IS NULL`

	// Using NamedArg for Oracle compatibility if needed, otherwise :quiz_id or $1 depending on driver
	// For sqlx, often the struct field names are used with :quiz_id if the arg is a struct,
	// or positional like :1, :2 if args are passed directly.
	// As quizID is a simple string, using :1 (positional) is appropriate here.
	err := a.db.GetContext(ctx, &modelEval, query, quizID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get quiz evaluation for quiz ID %s: %w", quizID, err)
	}
	return toDomainQuizEvaluation(&modelEval)
}

// GetRandomQuizBySubCategory implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuizBySubCategory(ctx context.Context, subCategoryID string) (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	query := `SELECT 
		id "ID",
		question "QUESTION",
		model_answers "MODEL_ANSWERS",
		keywords "KEYWORDS",
		difficulty "DIFFICULTY",
		sub_category_id "SUB_CATEGORY_ID",
		created_at "CREATED_AT",
		updated_at "UPDATED_AT",
		deleted_at "DELETED_AT"
	FROM quizzes
	WHERE sub_category_id = :1 
	AND deleted_at IS NULL
	ORDER BY DBMS_RANDOM.VALUE 
	FETCH FIRST 1 ROWS ONLY`

	err := a.db.GetContext(ctx, &modelQuiz, query, subCategoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No quiz found
		}
		return nil, fmt.Errorf("failed to get random quiz for sub-category ID %s: %w", subCategoryID, err)
	}
	return toDomainQuiz(&modelQuiz)
}

// GetQuizzesByCriteria implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizzesByCriteria(ctx context.Context, subCategoryID string, count int) ([]*domain.Quiz, error) {
	var modelQuizzes []models.Quiz
	query := `SELECT 
		id "ID",
		question "QUESTION",
		model_answers "MODEL_ANSWERS",
		keywords "KEYWORDS",
		difficulty "DIFFICULTY",
		sub_category_id "SUB_CATEGORY_ID",
		created_at "CREATED_AT",
		updated_at "UPDATED_AT",
		deleted_at "DELETED_AT"
	FROM quizzes 
	WHERE sub_category_id = :1 
	AND deleted_at IS NULL 
	ORDER BY DBMS_RANDOM.VALUE 
	FETCH FIRST :2 ROWS ONLY`

	err := a.db.SelectContext(ctx, &modelQuizzes, query, subCategoryID, count)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*domain.Quiz{}, nil
		}
		return nil, fmt.Errorf("failed to get quizzes by criteria: %w", err)
	}

	var quizzes []*domain.Quiz
	for _, modelQuiz := range modelQuizzes {
		quiz, convertErr := toDomainQuiz(&modelQuiz)
		if convertErr != nil {
			return nil, fmt.Errorf("failed to convert model to domain quiz: %w", convertErr)
		}
		quizzes = append(quizzes, quiz)
	}
	return quizzes, nil
}

// GetSubCategoryIDByName returns the ID of a subcategory by its name
func (a *QuizDatabaseAdapter) GetSubCategoryIDByName(ctx context.Context, name string) (string, error) {
	var subCategoryID string
	query := `SELECT id "ID" FROM sub_categories WHERE UPPER(name) = UPPER(:1) AND deleted_at IS NULL`

	err := a.db.GetContext(ctx, &subCategoryID, query, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get subcategory ID for name %s: %w", name, err)
	}

	return subCategoryID, nil
}

// GetQuizzesBySubCategory implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizzesBySubCategory(ctx context.Context, subCategoryID string) ([]*domain.Quiz, error) {
	var modelQuizzes []*models.Quiz
	query := `SELECT
		id "ID",
		question "QUESTION",
		model_answers "MODEL_ANSWERS",
		keywords "KEYWORDS",
		difficulty "DIFFICULTY",
		sub_category_id "SUB_CATEGORY_ID",
		created_at "CREATED_AT",
		updated_at "UPDATED_AT",
		deleted_at "DELETED_AT"
	FROM quizzes
	WHERE sub_category_id = :1
	AND deleted_at IS NULL
	ORDER BY created_at DESC`

	// Using SelectContext for context propagation
	err := a.db.SelectContext(ctx, &modelQuizzes, query, subCategoryID) // Already using SelectContext
	if err != nil {
		if err == sql.ErrNoRows {
			return []*domain.Quiz{}, nil
		}
		return nil, fmt.Errorf("failed to get quizzes by sub_category_id %s: %w", subCategoryID, err)
	}

	if len(modelQuizzes) == 0 {
		return []*domain.Quiz{}, nil
	}

	domainQuizzes := make([]*domain.Quiz, 0, len(modelQuizzes))
	for _, mq := range modelQuizzes {
		dq, err := toDomainQuiz(mq)
		if err != nil {
			return nil, fmt.Errorf("failed to convert model quiz (ID: %s) to domain quiz: %w", mq.ID, err)
		}
		domainQuizzes = append(domainQuizzes, dq)
	}

	return domainQuizzes, nil
}

// Helper functions for model conversion
func toDomainQuiz(m *models.Quiz) (*domain.Quiz, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot convert nil model.Quiz to domain.Quiz")
	}
	return &domain.Quiz{
		ID:            m.ID,
		Question:      m.Question,
		ModelAnswers:  strings.Split(m.ModelAnswers, stringDelimiter),
		Keywords:      strings.Split(m.Keywords, stringDelimiter),
		Difficulty:    m.Difficulty,
		SubCategoryID: m.SubCategoryID,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}, nil
}

func toModelQuiz(d *domain.Quiz) *models.Quiz {
	if d == nil {
		return nil
	}
	return &models.Quiz{
		ID:            d.ID,
		Question:      d.Question,
		ModelAnswers:  strings.Join(d.ModelAnswers, stringDelimiter),
		Keywords:      strings.Join(d.Keywords, stringDelimiter),
		Difficulty:    d.Difficulty,
		SubCategoryID: d.SubCategoryID,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func toModelAnswer(d *domain.Answer) *models.Answer {
	if d == nil {
		return nil
	}
	return &models.Answer{
		ID:             d.ID,
		QuizID:         d.QuizID,
		UserAnswer:     d.UserAnswer,
		Score:          d.Score,
		Explanation:    d.Explanation,
		KeywordMatches: models.StringSlice(d.KeywordMatches),
		Completeness:   d.Completeness,
		Relevance:      d.Relevance,
		Accuracy:       d.Accuracy,
		AnsweredAt:     d.AnsweredAt,
	}
}

func toModelQuizEvaluation(d *domain.QuizEvaluation) (*models.QuizEvaluation, error) {
	if d == nil {
		return nil, nil
	}

	scoreEvaluationsJSON, err := json.Marshal(d.ScoreEvaluations)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ScoreEvaluations: %w", err)
	}

	// SQL NullString 필드들을 적절히 설정
	var requiredTopics, scoreRanges, sampleAnswers, rubricDetails, scoreEvaluations sql.NullString

	if len(d.RequiredTopics) > 0 {
		requiredTopics = sql.NullString{String: strings.Join(d.RequiredTopics, stringDelimiter), Valid: true}
	}

	if len(d.ScoreRanges) > 0 {
		scoreRanges = sql.NullString{String: strings.Join(d.ScoreRanges, stringDelimiter), Valid: true}
	}

	if len(d.SampleAnswers) > 0 {
		sampleAnswers = sql.NullString{String: strings.Join(d.SampleAnswers, stringDelimiter), Valid: true}
	}

	if d.RubricDetails != "" {
		rubricDetails = sql.NullString{String: d.RubricDetails, Valid: true}
	}

	if len(scoreEvaluationsJSON) > 0 {
		scoreEvaluations = sql.NullString{String: string(scoreEvaluationsJSON), Valid: true}
	}

	return &models.QuizEvaluation{
		ID:               d.ID,
		QuizID:           d.QuizID,
		MinimumKeywords:  d.MinimumKeywords,
		RequiredTopics:   requiredTopics,
		ScoreRanges:      scoreRanges,
		SampleAnswers:    sampleAnswers,
		RubricDetails:    rubricDetails,
		ScoreEvaluations: scoreEvaluations,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}, nil
}

func toDomainQuizEvaluation(m *models.QuizEvaluation) (*domain.QuizEvaluation, error) {
	if m == nil {
		return nil, nil
	}

	var scoreEvaluations []domain.ScoreEvaluationDetail
	if m.ScoreEvaluations.Valid && m.ScoreEvaluations.String != "" {
		if err := json.Unmarshal([]byte(m.ScoreEvaluations.String), &scoreEvaluations); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ScoreEvaluations: %w", err)
		}
	}

	// NULL 값 처리를 위한 안전한 문자열 분할
	var requiredTopics, scoreRanges, sampleAnswers []string

	if m.RequiredTopics.Valid && m.RequiredTopics.String != "" {
		requiredTopics = strings.Split(m.RequiredTopics.String, stringDelimiter)
	}

	if m.ScoreRanges.Valid && m.ScoreRanges.String != "" {
		scoreRanges = strings.Split(m.ScoreRanges.String, stringDelimiter)
	}

	if m.SampleAnswers.Valid && m.SampleAnswers.String != "" {
		sampleAnswers = strings.Split(m.SampleAnswers.String, stringDelimiter)
	}

	rubricDetails := ""
	if m.RubricDetails.Valid {
		rubricDetails = m.RubricDetails.String
	}

	return &domain.QuizEvaluation{
		ID:               m.ID,
		QuizID:           m.QuizID,
		MinimumKeywords:  m.MinimumKeywords,
		RequiredTopics:   requiredTopics,
		ScoreRanges:      scoreRanges,
		SampleAnswers:    sampleAnswers,
		RubricDetails:    rubricDetails,
		ScoreEvaluations: scoreEvaluations,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}, nil
}

// GetUnattemptedQuizzesWithDetails fetches quizzes a user hasn't attempted, along with sub-category details.
func (a *QuizDatabaseAdapter) GetUnattemptedQuizzesWithDetails(ctx context.Context, userID string, limit int, optionalSubCategoryID string) ([]dto.QuizRecommendationItem, error) {
	if limit <= 0 {
		limit = 10
	}

	var recommendations []dto.QuizRecommendationItem

	params := map[string]interface{}{
		"user_id": userID,
		"limit":   limit,
	}

	query := `
	SELECT
		q.id AS quiz_id,
		q.question AS quiz_question,
		sc.name AS sub_category_name,
		q.difficulty
	FROM quizzes q
	JOIN sub_categories sc ON q.sub_category_id = sc.id
	LEFT JOIN user_quiz_attempts uqa ON q.id = uqa.quiz_id AND uqa.user_id = :user_id
	WHERE q.deleted_at IS NULL AND uqa.id IS NULL`

	if optionalSubCategoryID != "" {
		query += " AND q.sub_category_id = :sub_category_id"
		params["sub_category_id"] = optionalSubCategoryID
	}

	query += " ORDER BY DBMS_RANDOM.VALUE FETCH FIRST :limit ROWS ONLY"

	// Use NamedQuery for compatibility with both sqlx.DB and sqlx.Tx
	rows, err := a.db.NamedQuery(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query for unattempted quizzes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item dto.QuizRecommendationItem
		if err := rows.StructScan(&item); err != nil {
			return nil, fmt.Errorf("failed to scan unattempted quiz item: %w", err)
		}
		recommendations = append(recommendations, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for unattempted quizzes: %w", err)
	}
	if recommendations == nil {
		return []dto.QuizRecommendationItem{}, nil
	}
	return recommendations, nil
}
