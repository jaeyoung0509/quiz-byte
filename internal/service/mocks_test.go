package service

import (
	"context"
	// "testing" // Removed unused import
	"time"    // Import time if any mock methods use it

	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto" // Import dto if used by any mock method signatures

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

func (m *MockQuizRepository) GetQuizByID(id string) (*domain.Quiz, error) {
	args := m.Called(id)
	// Handle cases where Get(0) might be nil if the mock is set up to return nil for the quiz
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetRandomQuiz() (*domain.Quiz, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetRandomQuizBySubCategory(subCategory string) (*domain.Quiz, error) {
	args := m.Called(subCategory)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetSimilarQuiz(quizID string) (*domain.Quiz, error) {
	args := m.Called(quizID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) SaveAnswer(answer *domain.Answer) error {
	args := m.Called(answer)
	return args.Error(0)
}

func (m *MockQuizRepository) GetQuizzesByCriteria(SubCategoryID string, limit int) ([]*domain.Quiz, error) {
	args := m.Called(SubCategoryID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetSubCategoryIDByName(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

// --- MockCategoryRepository ---
type MockCategoryRepository struct {
	mock.Mock
}

func (m *MockCategoryRepository) GetAllCategories() ([]*domain.Category, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Category), args.Error(1)
}

func (m *MockCategoryRepository) GetSubCategories(categoryID string) ([]*domain.SubCategory, error) {
	args := m.Called(categoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SubCategory), args.Error(1)
}

func (m *MockCategoryRepository) SaveCategory(category *domain.Category) error {
	args := m.Called(category)
	return args.Error(0)
}

func (m *MockCategoryRepository) SaveSubCategory(subCategory *domain.SubCategory) error {
	args := m.Called(subCategory)
	return args.Error(0)
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
var _ domain.AnswerEvaluator = (*MockAnswerEvaluator)(nil)
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
