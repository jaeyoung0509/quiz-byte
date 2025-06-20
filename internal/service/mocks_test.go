package service

import (
	"context"
	// "testing" // Removed unused import
	"time" // Import time if any mock methods use it

	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"  // Import dto if used by any mock method signatures
	"quiz-byte/internal/port" // Added for AnswerEvaluator

	// Added for AnswerEvaluator
	"github.com/stretchr/testify/mock"
)

// --- MockQuizRepository ---
type MockQuizRepository struct {
	mock.Mock
}

func (m *MockQuizRepository) GetAllSubCategories(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockQuizRepository) GetQuizzesBySubCategory(ctx context.Context, subCategoryID string) ([]*domain.Quiz, error) {
	args := m.Called(ctx, subCategoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) SaveQuiz(ctx context.Context, quiz *domain.Quiz) error {
	args := m.Called(ctx, quiz)
	return args.Error(0)
}

func (m *MockQuizRepository) GetQuizByID(ctx context.Context, id string) (*domain.Quiz, error) {
	args := m.Called(ctx, id)
	// Handle cases where Get(0) might be nil if the mock is set up to return nil for the quiz
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetRandomQuiz(ctx context.Context) (*domain.Quiz, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetRandomQuizBySubCategory(ctx context.Context, subCategory string) (*domain.Quiz, error) {
	args := m.Called(ctx, subCategory)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetQuizEvaluation(ctx context.Context, quizID string) (*domain.QuizEvaluation, error) {
	args := m.Called(ctx, quizID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.QuizEvaluation), args.Error(1)
}

func (m *MockQuizRepository) SaveQuizEvaluation(ctx context.Context, evaluation *domain.QuizEvaluation) error {
	args := m.Called(ctx, evaluation)
	return args.Error(0)
}

func (m *MockQuizRepository) GetUnattemptedQuizzesWithDetails(ctx context.Context, userID string, limit int, subCategoryID string) ([]dto.QuizRecommendationItem, error) {
	args := m.Called(ctx, userID, limit, subCategoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]dto.QuizRecommendationItem), args.Error(1)
}

func (m *MockQuizRepository) GetSimilarQuiz(ctx context.Context, quizID string) (*domain.Quiz, error) {
	args := m.Called(ctx, quizID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) SaveAnswer(ctx context.Context, answer *domain.Answer) error {
	args := m.Called(ctx, answer)
	return args.Error(0)
}

func (m *MockQuizRepository) GetQuizzesByCriteria(ctx context.Context, SubCategoryID string, limit int) ([]*domain.Quiz, error) {
	args := m.Called(ctx, SubCategoryID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetSubCategoryIDByName(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockQuizRepository) UpdateQuiz(ctx context.Context, quiz *domain.Quiz) error {
	args := m.Called(ctx, quiz)
	return args.Error(0)
}

// --- MockCategoryRepository ---
type MockCategoryRepository struct {
	mock.Mock
}

func (m *MockCategoryRepository) GetAllCategories(ctx context.Context) ([]*domain.Category, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Category), args.Error(1)
}

func (m *MockCategoryRepository) GetSubCategories(ctx context.Context, categoryID string) ([]*domain.SubCategory, error) {
	args := m.Called(ctx, categoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SubCategory), args.Error(1)
}

func (m *MockCategoryRepository) SaveCategory(ctx context.Context, category *domain.Category) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockCategoryRepository) SaveSubCategory(ctx context.Context, subCategory *domain.SubCategory) error {
	args := m.Called(ctx, subCategory)
	return args.Error(0)
}

func (m *MockCategoryRepository) GetByName(ctx context.Context, name string) (*domain.Category, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Category), args.Error(1)
}

func (m *MockCategoryRepository) GetByNameAndCategoryID(ctx context.Context, name string, categoryID string) (*domain.SubCategory, error) {
	args := m.Called(ctx, name, categoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SubCategory), args.Error(1)
}

// --- MockEmbeddingService ---
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) Generate(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

// --- MockQuizGenerationService ---
type MockQuizGenerationService struct {
	mock.Mock
}

func (m *MockQuizGenerationService) GenerateQuizCandidates(
	ctx context.Context,
	subCategoryName string,
	existingKeywords []string,
	numQuestions int,
) ([]*domain.NewQuizData, error) {
	args := m.Called(ctx, subCategoryName, existingKeywords, numQuestions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.NewQuizData), args.Error(1)
}

func (m *MockQuizGenerationService) GenerateScoreEvaluationsForQuiz(ctx context.Context, quiz *domain.Quiz, scoreRanges []string) ([]domain.ScoreEvaluationDetail, error) {
	args := m.Called(ctx, quiz, scoreRanges)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.ScoreEvaluationDetail), args.Error(1)
}

// --- MockAnswerEvaluator ---
// (Moved from quiz_test.go - ensure it's not duplicated if already present from another file)
type MockAnswerEvaluator struct {
	mock.Mock
}

func (m *MockAnswerEvaluator) EvaluateAnswer(question, modelAnswer, userAnswer string, keywords []string) (*domain.Answer, error) {
	args := m.Called(question, modelAnswer, userAnswer, keywords)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Answer), args.Error(1)
}

// --- MockCache ---
// (Moved from quiz_test.go - ensure it's not duplicated if already present from another file)
// This MockCache is for the direct cache usage in QuizService (e.g. InvalidateQuizCache)
// It's different from MockAnswerCacheDomainCache used in answer_cache_test.go
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCache) HGet(ctx context.Context, key, field string) (string, error) {
	args := m.Called(ctx, key, field)
	return args.String(0), args.Error(1)
}

// HGetAll, HSet, Expire are part of domain.Cache but might not be used by all mocks.
// Add them if a specific test needs them for this general MockCache.
// For AnswerCacheService tests, MockAnswerCacheDomainCache has its own HGetAll, HSet, Expire.
func (m *MockCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockCache) HSet(ctx context.Context, key string, field string, value string) error {
	args := m.Called(ctx, key, field, value)
	return args.Error(0)
}

func (m *MockCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.Called(ctx, key, expiration)
	return args.Error(0)
}

// Ensure all required methods for interfaces are present in the mocks
var _ domain.QuizRepository = (*MockQuizRepository)(nil)
var _ domain.CategoryRepository = (*MockCategoryRepository)(nil)
var _ domain.EmbeddingService = (*MockEmbeddingService)(nil)
var _ domain.QuizGenerationService = (*MockQuizGenerationService)(nil)
var _ port.AnswerEvaluator = (*MockAnswerEvaluator)(nil)
var _ domain.Cache = (*MockCache)(nil) // For the general MockCache

// MockAnswerCacheService (moved from quiz_test.go)
type MockAnswerCacheService struct {
	mock.Mock
}

func (m *MockAnswerCacheService) GetAnswerFromCache(ctx context.Context, quizID string, userAnswerEmbedding []float32, userAnswerText string) (*dto.CheckAnswerResponse, error) {
	args := m.Called(ctx, quizID, userAnswerEmbedding, userAnswerText)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.CheckAnswerResponse), args.Error(1)
}

func (m *MockAnswerCacheService) PutAnswerToCache(ctx context.Context, quizID string, userAnswerText string, userAnswerEmbedding []float32, evaluation *dto.CheckAnswerResponse) error {
	args := m.Called(ctx, quizID, userAnswerText, userAnswerEmbedding, evaluation)
	return args.Error(0)
}

var _ AnswerCacheService = (*MockAnswerCacheService)(nil) // Corrected to service.AnswerCacheService

// --- MockTransactionManager ---

type MockTransactionManager struct {
	mock.Mock
}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}
