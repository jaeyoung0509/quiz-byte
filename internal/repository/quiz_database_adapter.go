package repository

import (
	"context"
	"database/sql"
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

	_, err := a.db.ExecContext(ctx, query, // Already using ExecContext, ensure ctx is passed
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
		Difficulty    int    `db:"difficulty"`
		SubCategoryID string `db:"sub_category_id"`
	}{}
	queryCurrent := `SELECT 
		difficulty "difficulty", 
		sub_category_id "sub_category_id" 
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
	query := `SELECT DISTINCT sub_category_id FROM quizzes WHERE sub_category_id IS NOT NULL AND deleted_at IS NULL ORDER BY sub_category_id ASC`
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
	return fmt.Errorf("SaveQuizEvaluation not yet implemented with SQLx")
}

// GetQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizEvaluation(ctx context.Context, quizID string) (*domain.QuizEvaluation, error) {
	return nil, fmt.Errorf("GetQuizEvaluation not yet implemented with SQLx")
}

// GetRandomQuizBySubCategory implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuizBySubCategory(ctx context.Context, subCategoryID string) (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	query := `SELECT 
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
	query := `SELECT id FROM sub_categories WHERE UPPER(name) = UPPER(:1) AND deleted_at IS NULL`

	err := a.db.GetContext(ctx, &subCategoryID, query, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Return empty string when not found, not an error
		}
		return "", fmt.Errorf("failed to get subcategory ID for name %s: %w", name, err)
	}

	return subCategoryID, nil
}

// GetQuizzesBySubCategory implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizzesBySubCategory(ctx context.Context, subCategoryID string) ([]*domain.Quiz, error) {
	var modelQuizzes []*models.Quiz
	query := `SELECT
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
	WHERE sub_category_id = :1
	AND deleted_at IS NULL
	ORDER BY created_at DESC` // Or any other order you prefer

	// Using SelectContext for context propagation
	err := a.db.SelectContext(ctx, &modelQuizzes, query, subCategoryID) // Already using SelectContext
	if err != nil {
		// sqlx.Select already returns an empty slice if no rows are found,
		// so explicit sql.ErrNoRows check might not be strictly necessary
		// unless you want to return a specific error or log it.
		// For this requirement, returning an empty slice and nil error is desired.
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
			// Log the error, but decide if you want to skip this quiz or return an error
			// For now, let's return the error, as a partial list might be misleading
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
		KeywordMatches: models.StringSlice(d.KeywordMatches), // StringSlice handles its own conversion
		Completeness:   d.Completeness,
		Relevance:      d.Relevance,
		Accuracy:       d.Accuracy,
		AnsweredAt:     d.AnsweredAt,
		// CreatedAt and UpdatedAt are set in the adapter before DB call,
		// and are not part of the domain.Answer struct.
	}
}

// GetUnattemptedQuizzesWithDetails fetches quizzes a user hasn't attempted, along with sub-category details.
// Returns a slice of dto.QuizRecommendationItem.
func (a *QuizDatabaseAdapter) GetUnattemptedQuizzesWithDetails(ctx context.Context, userID string, limit int, optionalSubCategoryID string) ([]dto.QuizRecommendationItem, error) {
	if limit <= 0 {
		limit = 10 // Default limit
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

	rows, err := a.db.NamedQueryContext(ctx, query, params)
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
