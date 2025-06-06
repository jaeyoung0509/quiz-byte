package service

import (
	"context"
	"errors"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks for UserRepository, UserQuizAttemptRepository, QuizRepository
// These would typically be in a mocks_test.go or similar, but defined here for brevity for this subtask.
// Re-using MockUserRepository from auth_service_test.go (conceptually).

// MockUserQuizAttemptRepository is a mock type for domain.UserQuizAttemptRepository
type MockUserQuizAttemptRepository struct {
	mock.Mock
}

func (m *MockUserQuizAttemptRepository) CreateAttempt(ctx context.Context, attempt *domain.UserQuizAttempt) error {
	args := m.Called(ctx, attempt)
	return args.Error(0)
}

func (m *MockUserQuizAttemptRepository) GetAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]domain.UserQuizAttempt, int, error) {
	args := m.Called(ctx, userID, filters, pagination)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.UserQuizAttempt), args.Int(1), args.Error(2)
}

func (m *MockUserQuizAttemptRepository) GetIncorrectAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]domain.UserQuizAttempt, int, error) {
	args := m.Called(ctx, userID, filters, pagination)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.UserQuizAttempt), args.Int(1), args.Error(2)
}

// Re-using MockQuizRepository from quiz_service_test.go (conceptually)

func TestUserService_GetUserProfile_Success(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	// userQuizAttemptRepo and quizRepo are not used in GetUserProfile, can be nil or use empty mocks
	var mockAttemptRepo *MockUserQuizAttemptRepository
	var mockQuizRepo *MockQuizRepository

	userService := NewUserService(mockUserRepo, mockAttemptRepo, mockQuizRepo)

	userID := "user1"
	expectedUser := &domain.User{ID: userID, Name: "Test User", Email: "test@example.com"}
	mockUserRepo.On("GetUserByID", mock.Anything, userID).Return(expectedUser, nil)

	profile, err := userService.GetUserProfile(context.Background(), userID)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, expectedUser.ID, profile.ID)
	assert.Equal(t, expectedUser.Name, profile.Name)
	mockUserRepo.AssertExpectations(t)
}

func TestUserService_GetUserProfile_NotFound(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	var mockAttemptRepo *MockUserQuizAttemptRepository
	var mockQuizRepo *MockQuizRepository
	userService := NewUserService(mockUserRepo, mockAttemptRepo, mockQuizRepo)

	userID := "unknownUser"
	// Simulate repository returning (nil, nil) for not found
	mockUserRepo.On("GetUserByID", mock.Anything, userID).Return(nil, nil)

	profile, err := userService.GetUserProfile(context.Background(), userID)

	assert.Error(t, err)
	assert.Nil(t, profile)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr), "Error should be a domain.DomainError")
	if domainErr != nil {
		assert.Equal(t, domain.CodeNotFound, domainErr.Code)
		assert.Contains(t, domainErr.Message, "user profile not found")
	}
	mockUserRepo.AssertExpectations(t)
}

func TestUserService_GetUserProfile_RepositoryError(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	var mockAttemptRepo *MockUserQuizAttemptRepository
	var mockQuizRepo *MockQuizRepository
	userService := NewUserService(mockUserRepo, mockAttemptRepo, mockQuizRepo)

	userID := "user1"
	expectedRepoError := errors.New("database connection error")
	// Simulate repository returning a generic error
	mockUserRepo.On("GetUserByID", mock.Anything, userID).Return(nil, expectedRepoError)

	profile, err := userService.GetUserProfile(context.Background(), userID)

	assert.Error(t, err)
	assert.Nil(t, profile)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr), "Error should be a domain.DomainError")
	if domainErr != nil {
		assert.Equal(t, domain.CodeInternal, domainErr.Code)
		assert.ErrorIs(t, err, expectedRepoError) // Check original error is wrapped
	}
	mockUserRepo.AssertExpectations(t)
}

func TestUserService_GetUserQuizAttempts_RepoError(t *testing.T) {
	mockUserRepo := new(MockUserRepository) // Not used directly in this method, but needed for service creation
	mockAttemptRepo := new(MockUserQuizAttemptRepository)
	mockQuizRepo := new(MockQuizRepository) // Not used if GetAttemptsByUserID fails first
	userService := NewUserService(mockUserRepo, mockAttemptRepo, mockQuizRepo)

	userID := "user1"
	filters := dto.AttemptFilters{}
	pagination := dto.Pagination{Limit: 10, Offset: 0}
	expectedRepoError := errors.New("attempt repo failure")

	mockAttemptRepo.On("GetAttemptsByUserID", mock.Anything, userID, filters, pagination).Return(nil, 0, expectedRepoError)

	_, err := userService.GetUserQuizAttempts(context.Background(), userID, filters, pagination)

	assert.Error(t, err)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr))
	if domainErr != nil {
		assert.Equal(t, domain.CodeInternal, domainErr.Code)
		assert.ErrorIs(t, err, expectedRepoError)
	}
	mockAttemptRepo.AssertExpectations(t)
}

func TestUserService_GetUserQuizAttempts_QuizDetailNotFound(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockAttemptRepo := new(MockUserQuizAttemptRepository)
	mockQuizRepo := new(MockQuizRepository)
	userService := NewUserService(mockUserRepo, mockAttemptRepo, mockQuizRepo)

	userID := "user1"
	filters := dto.AttemptFilters{}
	pagination := dto.Pagination{Limit: 10, Offset: 0}

	attemptTime := time.Now()
	domainAttempts := []domain.UserQuizAttempt{
		{ID: "attempt1", UserID: userID, QuizID: "quiz1", AttemptedAt: attemptTime},
	}

	mockAttemptRepo.On("GetAttemptsByUserID", mock.Anything, userID, filters, pagination).Return(domainAttempts, 1, nil)
	// Simulate QuizRepo.GetQuizByID returning (nil, nil) for not found
	mockQuizRepo.On("GetQuizByID", mock.Anything, "quiz1").Return(nil, nil)

	_, err := userService.GetUserQuizAttempts(context.Background(), userID, filters, pagination)

	assert.Error(t, err)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr))
	if domainErr != nil {
		assert.Equal(t, domain.CodeQuizNotFound, domainErr.Code)
	}
	mockAttemptRepo.AssertExpectations(t)
	mockQuizRepo.AssertExpectations(t)
}

func TestUserService_GetUserQuizAttempts_QuizDetailRepoError(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockAttemptRepo := new(MockUserQuizAttemptRepository)
	mockQuizRepo := new(MockQuizRepository)
	userService := NewUserService(mockUserRepo, mockAttemptRepo, mockQuizRepo)

	userID := "user1"
	filters := dto.AttemptFilters{}
	pagination := dto.Pagination{Limit: 10, Offset: 0}
	expectedRepoError := errors.New("quiz repo failure")

	attemptTime := time.Now()
	domainAttempts := []domain.UserQuizAttempt{
		{ID: "attempt1", UserID: userID, QuizID: "quiz1", AttemptedAt: attemptTime},
	}

	mockAttemptRepo.On("GetAttemptsByUserID", mock.Anything, userID, filters, pagination).Return(domainAttempts, 1, nil)
	mockQuizRepo.On("GetQuizByID", mock.Anything, "quiz1").Return(nil, expectedRepoError)

	_, err := userService.GetUserQuizAttempts(context.Background(), userID, filters, pagination)

	assert.Error(t, err)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr))
	if domainErr != nil {
		assert.Equal(t, domain.CodeInternal, domainErr.Code)
		assert.ErrorIs(t, err, expectedRepoError)
	}
	mockAttemptRepo.AssertExpectations(t)
	mockQuizRepo.AssertExpectations(t)
}

// TODO: Add tests for RecordQuizAttempt, GetUserIncorrectAnswers, GetUserRecommendations focusing on error paths.
