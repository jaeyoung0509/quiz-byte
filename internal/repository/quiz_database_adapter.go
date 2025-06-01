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
	query := `SELECT id, question, model_answers, keywords, difficulty, sub_category_id, created_at, updated_at, deleted_at FROM quizzes WHERE deleted_at IS NULL ORDER BY DBMS_RANDOM.VALUE FETCH FIRST 1 ROWS ONLY`
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
	query := `SELECT id, question, model_answers, keywords, difficulty, sub_category_id, created_at, updated_at, deleted_at FROM quizzes WHERE id = :id AND deleted_at IS NULL`
	// For sqlx with Oracle, named queries often use :named syntax in the query
	// and a map or struct for arguments.
	args := map[string]interface{}{"id": id}
	nstmt, err := a.db.PrepareNamed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare named statement for GetQuizByID: %w", err)
	}
	defer nstmt.Close()
	err = nstmt.Get(&modelQuiz, args)

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

	query := `INSERT INTO quizzes (id, question, model_answers, keywords, difficulty, sub_category_id, created_at, updated_at)
              VALUES (:id, :question, :model_answers, :keywords, :difficulty, :sub_category_id, :created_at, :updated_at)`
	_, err := a.db.NamedExec(query, modelQuiz)
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

	query := `UPDATE quizzes SET question = :question, model_answers = :model_answers, keywords = :keywords, difficulty = :difficulty, sub_category_id = :sub_category_id, updated_at = :updated_at
              WHERE id = :id AND deleted_at IS NULL`
	result, err := a.db.NamedExec(query, modelQuiz)
	if err != nil {
		return fmt.Errorf("failed to update quiz: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("quiz with ID %s not found or not updated", quiz.ID) // Or sql.ErrNoRows style error
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
	modelAnswer.AnsweredAt = time.Now() // Should be set by service layer ideally
	modelAnswer.CreatedAt = time.Now()
	modelAnswer.UpdatedAt = time.Now()

	query := `INSERT INTO answers (id, quiz_id, user_answer, score, explanation, keyword_matches, completeness, relevance, accuracy, answered_at, created_at, updated_at)
              VALUES (:id, :quiz_id, :user_answer, :score, :explanation, :keyword_matches, :completeness, :relevance, :accuracy, :answered_at, :created_at, :updated_at)`
	_, err := a.db.NamedExec(query, modelAnswer)
	if err != nil {
		return fmt.Errorf("failed to save answer: %w", err)
	}
	answer.ID = modelAnswer.ID
	answer.AnsweredAt = modelAnswer.AnsweredAt
	// domain.Answer does not have CreatedAt/UpdatedAt
	return nil
}

// GetSimilarQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetSimilarQuiz(quizID string) (*domain.Quiz, error) {
	current := struct {
		Difficulty    int    `db:"difficulty"`
		SubCategoryID string `db:"sub_category_id"`
	}{}
	queryCurrent := `SELECT difficulty, sub_category_id FROM quizzes WHERE id = :id AND deleted_at IS NULL`

	nstmtCurrent, err := a.db.PrepareNamed(queryCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare named statement for GetSimilarQuiz (current): %w", err)
	}
	defer nstmtCurrent.Close()
	err = nstmtCurrent.Get(&current, map[string]interface{}{"id": quizID})

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get current quiz for similarity check: %w", err)
	}

	var similarQuizModel models.Quiz
	querySimilar := `SELECT id, question, model_answers, keywords, difficulty, sub_category_id, created_at, updated_at, deleted_at
                     FROM quizzes
                     WHERE id != :current_id AND sub_category_id = :sub_category_id AND difficulty = :difficulty AND deleted_at IS NULL
                     ORDER BY DBMS_RANDOM.VALUE FETCH FIRST 1 ROWS ONLY`

	argsSimilar := map[string]interface{}{
		"current_id":      quizID,
		"sub_category_id": current.SubCategoryID,
		"difficulty":      current.Difficulty,
	}
	nstmtSimilar, err := a.db.PrepareNamed(querySimilar)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare named statement for GetSimilarQuiz (similar): %w", err)
	}
	defer nstmtSimilar.Close()
	err = nstmtSimilar.Get(&similarQuizModel, argsSimilar)

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
	query := `SELECT id, question, model_answers, keywords, difficulty, sub_category_id, created_at, updated_at, deleted_at
              FROM quizzes
              WHERE sub_category_id = :sub_category_id AND deleted_at IS NULL
              ORDER BY DBMS_RANDOM.VALUE FETCH FIRST 1 ROWS ONLY`

	args := map[string]interface{}{"sub_category_id": subCategoryID}
	nstmt, err := a.db.PrepareNamed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare named statement for GetRandomQuizBySubCategory: %w", err)
	}
	defer nstmt.Close()
	err = nstmt.Get(&modelQuiz, args)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get random quiz for sub-category ID %s: %w", subCategoryID, err)
	}
	return toDomainQuiz(&modelQuiz)
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
