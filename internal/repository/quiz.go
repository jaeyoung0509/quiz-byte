package repository

import (
	"fmt"
	"quiz-byte/internal/repository/models"

	"gorm.io/gorm"
)

type QuizRepository interface {
	GetRandomQuiz() (*models.Quiz, error)
	GetQuizByID(id int64) (*models.Quiz, error)
	SaveQuiz(quiz *models.Quiz) error
	SaveAnswer(answer *models.Answer) error
	GetSimilarQuiz(quizID int64) (*models.Quiz, error)
	GetAllSubCategories() ([]int64, error)
	GetRandomQuizBySubCategory(subCategoryID int64) (*models.Quiz, error)
}

type quizRepository struct {
	db *gorm.DB
}

func NewQuizRepository(db *gorm.DB) QuizRepository {
	return &quizRepository{db: db}
}

func (r *quizRepository) SaveQuiz(quiz *models.Quiz) error {
	if quiz == nil {
		return fmt.Errorf("quiz cannot be nil")
	}
	err := r.db.Create(quiz).Error
	if err != nil {
		return fmt.Errorf("failed to save quiz: %v", err)
	}
	return nil
}

func (r *quizRepository) GetAllSubCategories() ([]int64, error) {
	var categories []int64
	err := r.db.Model(&models.Quiz{}).
		Distinct().
		Pluck("sub_category_id", &categories).
		Error
	if err != nil {
		return nil, fmt.Errorf("failed to query sub categories: %v", err)
	}
	return categories, nil
}

func (r *quizRepository) GetRandomQuizBySubCategory(subCategoryID int64) (*models.Quiz, error) {
	var quiz models.Quiz
	err := r.db.Where("sub_category_id = ?", subCategoryID).
		Order("DBMS_RANDOM.VALUE").
		Take(&quiz).Error
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("no quiz found for sub-category ID: %d", subCategoryID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get random quiz: %v", err)
	}
	return &quiz, nil
}

func (r *quizRepository) GetQuizByID(id int64) (*models.Quiz, error) {
	var quiz models.Quiz
	err := r.db.Where("id = ?", id).First(&quiz).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get quiz: %v", err)
	}
	return &quiz, nil
}

func (r *quizRepository) GetRandomQuiz() (*models.Quiz, error) {
	var quiz models.Quiz
	err := r.db.Order("DBMS_RANDOM.VALUE").First(&quiz).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get random quiz: %v", err)
	}
	return &quiz, nil
}

func (r *quizRepository) SaveAnswer(answer *models.Answer) error {
	if answer == nil {
		return fmt.Errorf("answer cannot be nil")
	}
	err := r.db.Create(answer).Error
	if err != nil {
		return fmt.Errorf("failed to save answer: %v", err)
	}
	return nil
}

func (r *quizRepository) GetSimilarQuiz(quizID int64) (*models.Quiz, error) {
	current := &models.Quiz{}
	err := r.db.First(current, quizID).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get current quiz: %v", err)
	}

	// 현재 퀴즈와 같은 난이도의 다른 퀴즈를 랜덤하게 선택
	var similar models.Quiz
	err = r.db.Where("id != ? AND difficulty = ?", quizID, current.Difficulty).
		Order("RANDOM()").
		First(&similar).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get similar quiz: %v", err)
	}
	return &similar, nil
}
