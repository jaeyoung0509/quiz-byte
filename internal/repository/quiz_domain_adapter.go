package repository

import (
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"strings"
	"time"
)

// QuizDomainAdapter adapts the repository.QuizRepository (GORM-specific) to domain.QuizRepository
type QuizDomainAdapter struct {
	repo QuizRepository // This is the GORM-specific repository.QuizRepository
}

// NewQuizDomainRepositoryAdapter creates a new instance of QuizDomainAdapter
func NewQuizDomainRepositoryAdapter(repo QuizRepository) domain.QuizRepository {
	return &QuizDomainAdapter{repo: repo}
}

// GetRandomQuiz implements domain.QuizRepository
func (a *QuizDomainAdapter) GetRandomQuiz() (*domain.Quiz, error) {
	quiz, err := a.repo.GetRandomQuiz()
	if err != nil {
		return nil, err
	}
	if quiz == nil {
		return nil, nil
	}
	return toDomainQuiz(quiz)
}

// GetQuizByID implements domain.QuizRepository
func (a *QuizDomainAdapter) GetQuizByID(id int64) (*domain.Quiz, error) {
	quiz, err := a.repo.GetQuizByID(id)
	if err != nil {
		return nil, err
	}
	if quiz == nil {
		return nil, nil
	}
	return toDomainQuiz(quiz)
}

// SaveQuiz implements domain.QuizRepository
func (a *QuizDomainAdapter) SaveQuiz(quiz *domain.Quiz) error {
	return a.repo.SaveQuiz(toModelQuiz(quiz))
}

// UpdateQuiz implements domain.QuizRepository
func (a *QuizDomainAdapter) UpdateQuiz(quiz *domain.Quiz) error {
	return a.repo.SaveQuiz(toModelQuiz(quiz))
}

// SaveAnswer implements domain.QuizRepository
func (a *QuizDomainAdapter) SaveAnswer(answer *domain.Answer) error {
	return a.repo.SaveAnswer(toModelAnswer(answer))
}

// GetSimilarQuiz implements domain.QuizRepository
func (a *QuizDomainAdapter) GetSimilarQuiz(quizID int64) (*domain.Quiz, error) {
	quiz, err := a.repo.GetSimilarQuiz(quizID)
	if err != nil {
		return nil, err
	}
	if quiz == nil {
		return nil, nil
	}
	return toDomainQuiz(quiz)
}

// GetAllSubCategories implements domain.QuizRepository
func (a *QuizDomainAdapter) GetAllSubCategories() ([]string, error) {
	// 현재는 더미 데이터 반환
	return []string{"general", "programming", "database", "algorithms"}, nil
}

// SaveQuizEvaluation implements domain.QuizRepository
func (a *QuizDomainAdapter) SaveQuizEvaluation(evaluation *domain.QuizEvaluation) error {
	// TODO: 구현 예정
	return nil
}

// GetQuizEvaluation implements domain.QuizRepository
func (a *QuizDomainAdapter) GetQuizEvaluation(quizID int64) (*domain.QuizEvaluation, error) {
	// TODO: 구현 예정
	return nil, nil
}

// GetRandomQuizBySubCategory implements domain.QuizRepository
func (a *QuizDomainAdapter) GetRandomQuizBySubCategory(subCategoryID int64) (*domain.Quiz, error) {
	quiz, err := a.repo.GetRandomQuiz() // 임시로 GetRandomQuiz 사용
	if err != nil {
		return nil, err
	}
	if quiz == nil {
		return nil, nil
	}
	return toDomainQuiz(quiz)
}

// Helper functions for model conversion
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
