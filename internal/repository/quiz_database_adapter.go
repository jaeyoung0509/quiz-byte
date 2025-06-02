package repository

import (
	"database/sql"
	"fmt"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const stringDelimiter = "|||"

// QuizDatabaseAdapter implements domain.QuizRepository using sqlx.DB
type QuizDatabaseAdapter struct {
	db *sqlx.DB
}

// NewQuizDatabaseAdapter creates a new instance of QuizDatabaseAdapter
func NewQuizDatabaseAdapter(db *sqlx.DB) domain.QuizRepository {
	return &QuizDatabaseAdapter{db: db}
}

// GetRandomQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuiz() (*domain.Quiz, error) {
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

	// Oracle에서는 RowsAffected가 항상 0을 반환하므로, Get을 직접 사용
	err := a.db.Get(&modelQuiz, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get random quiz: %w", err)
	}
	return toDomainQuiz(&modelQuiz)
}

// GetQuizByID implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizByID(id string) (*domain.Quiz, error) {
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

	nstmt, err := a.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement for GetQuizByID: %w", err)
	}
	defer nstmt.Close()
	err = nstmt.QueryRow(id).Scan(
		&modelQuiz.ID,
		&modelQuiz.Question,
		&modelQuiz.ModelAnswers,
		&modelQuiz.Keywords,
		&modelQuiz.Difficulty,
		&modelQuiz.SubCategoryID,
		&modelQuiz.CreatedAt,
		&modelQuiz.UpdatedAt,
		&modelQuiz.DeletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get quiz by ID %s: %w", id, err)
	}
	return toDomainQuiz(&modelQuiz)
}

// SaveQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveQuiz(quiz *domain.Quiz) error {
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

	_, err := a.db.Exec(query,
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
func (a *QuizDatabaseAdapter) UpdateQuiz(quiz *domain.Quiz) error {
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

	result, err := a.db.Exec(query,
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
func (a *QuizDatabaseAdapter) SaveAnswer(answer *domain.Answer) error {
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

	_, err := a.db.Exec(query,
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
func (a *QuizDatabaseAdapter) GetSimilarQuiz(quizID string) (*domain.Quiz, error) {
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

	nstmt, err := a.db.Prepare(queryCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement for GetSimilarQuiz (current): %w", err)
	}
	defer nstmt.Close()
	err = nstmt.QueryRow(quizID).Scan(&current.Difficulty, &current.SubCategoryID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
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

	nstmtSimilar, err := a.db.Prepare(querySimilar)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement for GetSimilarQuiz (similar): %w", err)
	}
	defer nstmtSimilar.Close()
	err = nstmtSimilar.QueryRow(quizID, current.SubCategoryID, current.Difficulty).Scan(
		&similarQuizModel.ID,
		&similarQuizModel.Question,
		&similarQuizModel.ModelAnswers,
		&similarQuizModel.Keywords,
		&similarQuizModel.Difficulty,
		&similarQuizModel.SubCategoryID,
		&similarQuizModel.CreatedAt,
		&similarQuizModel.UpdatedAt,
		&similarQuizModel.DeletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get similar quiz: %w", err)
	}
	return toDomainQuiz(&similarQuizModel)
}

// GetAllSubCategories implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetAllSubCategories() ([]string, error) {
	var subCategoryIDs []string
	query := `SELECT DISTINCT sub_category_id FROM quizzes WHERE sub_category_id IS NOT NULL AND deleted_at IS NULL ORDER BY sub_category_id ASC`
	err := a.db.Select(&subCategoryIDs, query)
	if err != nil {
		// Select itself returns an empty slice if no rows are found and the destination is a slice.
		// So, ErrNoRows check is usually not needed here.
		// The main concern is other types of errors.
		return nil, fmt.Errorf("failed to query sub categories: %w", err)
	}
	if subCategoryIDs == nil {
		return []string{}, nil // Ensure non-nil empty slice for no results.
	}
	return subCategoryIDs, nil
}

// SaveQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveQuizEvaluation(evaluation *domain.QuizEvaluation) error {
	return fmt.Errorf("SaveQuizEvaluation not yet implemented with SQLx")
}

// GetQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizEvaluation(quizID string) (*domain.QuizEvaluation, error) {
	return nil, fmt.Errorf("GetQuizEvaluation not yet implemented with SQLx")
}

// GetRandomQuizBySubCategory implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuizBySubCategory(subCategoryID string) (*domain.Quiz, error) {
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

	nstmt, err := a.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement for GetRandomQuizBySubCategory: %w", err)
	}
	defer nstmt.Close()
	err = nstmt.QueryRow(subCategoryID).Scan(
		&modelQuiz.ID,
		&modelQuiz.Question,
		&modelQuiz.ModelAnswers,
		&modelQuiz.Keywords,
		&modelQuiz.Difficulty,
		&modelQuiz.SubCategoryID,
		&modelQuiz.CreatedAt,
		&modelQuiz.UpdatedAt,
		&modelQuiz.DeletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get random quiz for sub-category ID %s: %w", subCategoryID, err)
	}
	return toDomainQuiz(&modelQuiz)
}

// GetQuizzesByCriteria implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizzesByCriteria(subCategoryID string, count int) ([]*domain.Quiz, error) {
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
	ORDER BY DBMS_RANDOM.VALUE
	FETCH FIRST :2 ROWS ONLY`

	// Using NamedQuery for clarity with multiple parameters
	rows, err := a.db.NamedQuery(query, map[string]interface{}{"1": subCategoryID, "2": count})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query for GetQuizzesByCriteria: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var modelQuiz models.Quiz
		if err := rows.StructScan(&modelQuiz); err != nil {
			return nil, fmt.Errorf("failed to scan quiz row: %w", err)
		}
		modelQuizzes = append(modelQuizzes, &modelQuiz)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for GetQuizzesByCriteria: %w", err)
	}

	if len(modelQuizzes) == 0 {
		return []*domain.Quiz{}, nil // Return empty slice, not nil, if no quizzes found
	}

	domainQuizzes := make([]*domain.Quiz, 0, len(modelQuizzes))
	for _, mq := range modelQuizzes {
		dq, err := toDomainQuiz(mq)
		if err != nil {
			// Log or handle individual conversion errors if necessary
			return nil, fmt.Errorf("failed to convert model quiz to domain quiz: %w", err)
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
