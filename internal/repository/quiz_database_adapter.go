package repository

import (
	"fmt"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

// QuizDatabaseAdapter implements domain.QuizRepository using gorm.DB
type QuizDatabaseAdapter struct {
	db *gorm.DB
}

// NewQuizDatabaseAdapter creates a new instance of QuizDatabaseAdapter
func NewQuizDatabaseAdapter(db *gorm.DB) domain.QuizRepository {
	return &QuizDatabaseAdapter{db: db}
}

// GetRandomQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuiz() (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	err := a.db.Order("DBMS_RANDOM.VALUE").First(&modelQuiz).Error // Oracle specific random
	// For PostgreSQL or MySQL, use: a.db.Order("RANDOM()").First(&modelQuiz).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Or domain.NewQuizNotFoundError if appropriate
		}
		return nil, fmt.Errorf("failed to get random quiz: %w", err)
	}
	return toDomainQuiz(&modelQuiz)
}

// GetQuizByID implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizByID(id int64) (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	err := a.db.Where("id = ?", id).First(&modelQuiz).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Or domain.NewQuizNotFoundError(id)
		}
		return nil, fmt.Errorf("failed to get quiz by ID %d: %w", id, err)
	}
	return toDomainQuiz(&modelQuiz)
}

// SaveQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveQuiz(quiz *domain.Quiz) error {
	modelQuiz := toModelQuiz(quiz)
	if modelQuiz == nil {
		return fmt.Errorf("cannot save nil quiz")
	}
	err := a.db.Create(modelQuiz).Error
	if err != nil {
		return fmt.Errorf("failed to save quiz: %w", err)
	}
	quiz.ID = modelQuiz.ID // Update domain model with new ID
	return nil
}

// UpdateQuiz implements domain.QuizRepository
// Note: GORM's Save updates all fields or creates if ID is zero.
// If specific update logic is needed (e.g. only certain fields), this should be more specific.
func (a *QuizDatabaseAdapter) UpdateQuiz(quiz *domain.Quiz) error {
	modelQuiz := toModelQuiz(quiz)
	if modelQuiz == nil {
		return fmt.Errorf("cannot update nil quiz")
	}
	if modelQuiz.ID == 0 {
		return fmt.Errorf("cannot update quiz with zero ID")
	}
	// Save will update the record if it has a primary key, or insert it if it does not.
	err := a.db.Save(modelQuiz).Error
	if err != nil {
		return fmt.Errorf("failed to update quiz: %w", err)
	}
	return nil
}

// SaveAnswer implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveAnswer(answer *domain.Answer) error {
	modelAnswer := toModelAnswer(answer)
	if modelAnswer == nil {
		return fmt.Errorf("cannot save nil answer")
	}
	err := a.db.Create(modelAnswer).Error
	if err != nil {
		return fmt.Errorf("failed to save answer: %w", err)
	}
	answer.ID = modelAnswer.ID // Update domain model with new ID
	return nil
}

// GetSimilarQuiz implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetSimilarQuiz(quizID int64) (*domain.Quiz, error) {
	current := &models.Quiz{}
	err := a.db.First(current, quizID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Or specific error
		}
		return nil, fmt.Errorf("failed to get current quiz for similarity check: %w", err)
	}

	var similarQuiz models.Quiz
	// Example: find another quiz with the same difficulty, excluding the current one
	err = a.db.Where("id != ? AND difficulty = ?", quizID, current.Difficulty).
		Order("DBMS_RANDOM.VALUE"). // Oracle specific
		// For PostgreSQL or MySQL, use: Order("RANDOM()")
		First(&similarQuiz).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get similar quiz: %w", err)
	}
	return toDomainQuiz(&similarQuiz)
}

// GetAllSubCategories implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetAllSubCategories() ([]string, error) {
	// Assuming sub_category_id in models.Quiz is actually the subcategory name or maps to it.
	// If it's an ID that needs to be looked up, this query needs adjustment.
	// For this example, let's assume 'sub_category_id' itself is a string representing the name.
	// If 'sub_category_id' is an int ID, you'd query a different table or join.
	// The original GORM code returned []int64, but the adapter was already (correctly) returning []string.
	// This implementation will be a placeholder as the previous one was.
	// A proper implementation would query distinct subcategory names from the quizzes table or a dedicated subcategories table.
	// For now, retaining the placeholder logic from the previous adapter version:
	// log.Println("Warning: GetAllSubCategories is returning dummy data.")
	return []string{"general", "programming", "database", "algorithms"}, nil
	/*
		// Example of a more realistic query if sub_category_name was a field:
		var subCategoryNames []string
		err := a.db.Model(&models.Quiz{}).Distinct().Pluck("sub_category_name", &subCategoryNames).Error
		if err != nil {
			return nil, fmt.Errorf("failed to query sub categories: %w", err)
		}
		return subCategoryNames, nil
	*/
}

// SaveQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) SaveQuizEvaluation(evaluation *domain.QuizEvaluation) error {
	// TODO: Implement using a.db and model conversion if needed
	// Example: modelEvaluation := toModelQuizEvaluation(evaluation)
	// err := a.db.Create(modelEvaluation).Error
	return fmt.Errorf("SaveQuizEvaluation not yet implemented")
}

// GetQuizEvaluation implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetQuizEvaluation(quizID int64) (*domain.QuizEvaluation, error) {
	// TODO: Implement using a.db and model conversion if needed
	// Example: var modelEval models.QuizEvaluation
	// err := a.db.Where("quiz_id = ?", quizID).First(&modelEval).Error
	// return toDomainQuizEvaluation(&modelEval), err
	return nil, fmt.Errorf("GetQuizEvaluation not yet implemented")
}

// GetRandomQuizBySubCategory implements domain.QuizRepository
func (a *QuizDatabaseAdapter) GetRandomQuizBySubCategory(subCategoryID int64) (*domain.Quiz, error) {
	var modelQuiz models.Quiz
	// Assuming subCategoryID in domain.Quiz maps to a field like 'sub_category_id' in models.Quiz
	err := a.db.Where("sub_category_id = ?", subCategoryID).
		Order("DBMS_RANDOM.VALUE"). // Oracle specific
		// For PostgreSQL or MySQL, use: Order("RANDOM()")
		Take(&modelQuiz).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Or domain.NewQuizNotFoundError for subcategory
		}
		return nil, fmt.Errorf("failed to get random quiz for sub-category ID %d: %w", subCategoryID, err)
	}
	return toDomainQuiz(&modelQuiz)
}

// Helper functions for model conversion (already present, ensure they are correct)
func toDomainQuiz(m *models.Quiz) (*domain.Quiz, error) {
	return &domain.Quiz{
		ID:           m.ID,
		Question:     m.Question,
		ModelAnswers: strings.Split(m.ModelAnswers, ","),
		Keywords:     strings.Split(m.Keywords, ","),
		Difficulty:   m.Difficulty, // Already an int, no conversion needed
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}, nil
}

func toModelQuiz(d *domain.Quiz) *models.Quiz {
	return &models.Quiz{
		ID:           d.ID,
		Question:     d.Question,
		ModelAnswers: strings.Join(d.ModelAnswers, ","),
		Keywords:     strings.Join(d.Keywords, ","),
		Difficulty:   d.Difficulty, // Directly use the int value
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

func toModelAnswer(d *domain.Answer) *models.Answer {
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
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}
