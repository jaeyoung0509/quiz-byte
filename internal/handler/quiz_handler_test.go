package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/handler"
	"quiz-byte/internal/middleware"

	// "quiz-byte/internal/util" // Not directly used in test, but by code under test
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// --- Manual Mocks ---

// MockQuizService
type MockQuizService struct {
	GetRandomQuizFunc       func(subCategory string) (*dto.QuizResponse, error)
	CheckAnswerFunc         func(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error)
	GetAllSubCategoriesFunc func() ([]string, error)
	GetBulkQuizzesFunc      func(req *dto.BulkQuizzesRequest) (*dto.BulkQuizzesResponse, error)
}

func (m *MockQuizService) GetRandomQuiz(subCategory string) (*dto.QuizResponse, error) {
	if m.GetRandomQuizFunc != nil {
		return m.GetRandomQuizFunc(subCategory)
	}
	panic("MockQuizService.GetRandomQuizFunc not implemented")
}
func (m *MockQuizService) CheckAnswer(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error) {
	if m.CheckAnswerFunc != nil {
		return m.CheckAnswerFunc(req)
	}
	panic("MockQuizService.CheckAnswerFunc not implemented")
}
func (m *MockQuizService) GetAllSubCategories() ([]string, error) {
	if m.GetAllSubCategoriesFunc != nil {
		return m.GetAllSubCategoriesFunc()
	}
	panic("MockQuizService.GetAllSubCategoriesFunc not implemented")
}
func (m *MockQuizService) GetBulkQuizzes(req *dto.BulkQuizzesRequest) (*dto.BulkQuizzesResponse, error) {
	if m.GetBulkQuizzesFunc != nil {
		return m.GetBulkQuizzesFunc(req)
	}
	panic("MockQuizService.GetBulkQuizzesFunc not implemented")
}

// MockUserService
type MockUserService struct {
	GetUserProfileFunc          func(ctx context.Context, userID string) (*dto.UserProfileResponse, error)
	RecordQuizAttemptFunc       func(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error
	GetUserQuizAttemptsFunc     func(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserQuizAttemptsResponse, error)
	GetUserIncorrectAnswersFunc func(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserIncorrectAnswersResponse, error)
	GetUserRecommendationsFunc  func(ctx context.Context, userID string, limit int, optionalSubCategoryID string) (*dto.QuizRecommendationsResponse, error)
}

func (m *MockUserService) GetUserProfile(ctx context.Context, userID string) (*dto.UserProfileResponse, error) {
	if m.GetUserProfileFunc != nil {
		return m.GetUserProfileFunc(ctx, userID)
	}
	panic("MockUserService.GetUserProfileFunc not implemented")
}
func (m *MockUserService) RecordQuizAttempt(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error {
	if m.RecordQuizAttemptFunc != nil {
		return m.RecordQuizAttemptFunc(ctx, userID, quizID, userAnswer, evalResult)
	}
	panic("MockUserService.RecordQuizAttemptFunc not implemented")
}
func (m *MockUserService) GetUserQuizAttempts(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserQuizAttemptsResponse, error) {
	if m.GetUserQuizAttemptsFunc != nil {
		return m.GetUserQuizAttemptsFunc(ctx, userID, filters, pagination)
	}
	panic("MockUserService.GetUserQuizAttemptsFunc not implemented")
}
func (m *MockUserService) GetUserIncorrectAnswers(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserIncorrectAnswersResponse, error) {
	if m.GetUserIncorrectAnswersFunc != nil {
		return m.GetUserIncorrectAnswersFunc(ctx, userID, filters, pagination)
	}
	panic("MockUserService.GetUserIncorrectAnswersFunc not implemented")
}
func (m *MockUserService) GetUserRecommendations(ctx context.Context, userID string, limit int, optionalSubCategoryID string) (*dto.QuizRecommendationsResponse, error) {
	if m.GetUserRecommendationsFunc != nil {
		return m.GetUserRecommendationsFunc(ctx, userID, limit, optionalSubCategoryID)
	}
	panic("MockUserService.GetUserRecommendationsFunc not implemented")
}

// MockAnonymousResultCacheService
type MockAnonymousResultCacheService struct {
	PutFunc func(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error
	GetFunc func(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error)
}

func (m *MockAnonymousResultCacheService) Put(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
	if m.PutFunc != nil {
		return m.PutFunc(ctx, requestID, result)
	}
	panic("MockAnonymousResultCacheService.PutFunc not implemented")
}
func (m *MockAnonymousResultCacheService) Get(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, requestID)
	}
	panic("MockAnonymousResultCacheService.GetFunc not implemented")
}

func TestQuizHandler_CheckAnswer(t *testing.T) {
	var mockQuizSvc *MockQuizService
	var mockUserSvc *MockUserService
	var mockAnonCacheSvc *MockAnonymousResultCacheService
	var quizHandler *handler.QuizHandler

	setup := func() {
		mockQuizSvc = &MockQuizService{}
		mockUserSvc = &MockUserService{}
		mockAnonCacheSvc = &MockAnonymousResultCacheService{}
		quizHandler = handler.NewQuizHandler(mockQuizSvc, mockUserSvc, mockAnonCacheSvc)
	}

	// Generate a valid ULID for QuizID
	validQuizID := "01HGZ8VNRYXS8QKNJV5GRWPWDQ" // Valid ULID format for testing
	commonCheckAnswerRequest := dto.CheckAnswerRequest{
		QuizID:     validQuizID,
		UserAnswer: "My answer",
	}
	commonDomainResult := &dto.CheckAnswerResponse{
		Score:       0.8,
		Explanation: "Good work!",
		// Fill other fields as necessary, matching what quizService.CheckAnswer would return
		KeywordMatches: []string{"keyword1"},
		Completeness:   0.9,
		Relevance:      0.85,
		Accuracy:       0.75,
		ModelAnswer:    "This is the model answer.",
	}

	t.Run("Authenticated User", func(t *testing.T) {
		setup()
		userID := "userTest123"
		recordAttemptCalled := false
		var recordedUserID, recordedQuizID, recordedUserAnswer string

		mockQuizSvc.CheckAnswerFunc = func(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error) {
			assert.Equal(t, commonCheckAnswerRequest.QuizID, req.QuizID)
			assert.Equal(t, commonCheckAnswerRequest.UserAnswer, req.UserAnswer)
			return commonDomainResult, nil
		}
		mockUserSvc.RecordQuizAttemptFunc = func(ctx context.Context, uID string, qID string, uAnswer string, evalResult *domain.Answer) error {
			recordAttemptCalled = true
			recordedUserID = uID
			recordedQuizID = qID
			recordedUserAnswer = uAnswer

		// It's crucial to ensure evalResult is not nil before dereferencing.
		if !assert.NotNil(t, evalResult, "evalResult passed to RecordQuizAttempt should not be nil") {
			return errors.New("evalResult was nil and assertion failed") // Return error to stop further processing if nil
		}

		// Assuming commonCheckAnswerRequest contains the QuizID and UserAnswer that should be in evalResult
		// commonDomainResult is the *dto.CheckAnswerResponse
		assert.Equal(t, commonCheckAnswerRequest.QuizID, evalResult.QuizID, "evalResult.QuizID mismatch")
		assert.Equal(t, commonCheckAnswerRequest.UserAnswer, evalResult.UserAnswer, "evalResult.UserAnswer mismatch")
		assert.Equal(t, commonDomainResult.Score, evalResult.Score, "evalResult.Score mismatch")
		assert.Equal(t, commonDomainResult.Explanation, evalResult.Explanation, "evalResult.Explanation mismatch")
		assert.ElementsMatch(t, commonDomainResult.KeywordMatches, evalResult.KeywordMatches, "evalResult.KeywordMatches mismatch") // Use ElementsMatch for slices where order doesn't matter
		assert.Equal(t, commonDomainResult.Completeness, evalResult.Completeness, "evalResult.Completeness mismatch")
		assert.Equal(t, commonDomainResult.Relevance, evalResult.Relevance, "evalResult.Relevance mismatch")
		assert.Equal(t, commonDomainResult.Accuracy, evalResult.Accuracy, "evalResult.Accuracy mismatch")
		assert.WithinDuration(t, time.Now(), evalResult.AnsweredAt, 5*time.Second, "evalResult.AnsweredAt out of range")

			return nil
		}
		// Ensure Put is not called by setting it to fail the test if it is
		mockAnonCacheSvc.PutFunc = func(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
			assert.Fail(t, "AnonymousResultCacheService.Put should not be called for authenticated user")
			return errors.New("Put should not be called")
		}

		// Create a fiber app and use the test handler
		app := fiber.New(fiber.Config{
			ErrorHandler: middleware.ErrorHandler(),
		})
		app.Post("/quiz/check", func(c *fiber.Ctx) error {
			c.Locals(middleware.UserIDKey, userID)
			return quizHandler.CheckAnswer(c)
		})

		reqBodyBytes, _ := json.Marshal(commonCheckAnswerRequest)
		req := httptest.NewRequest("POST", "/quiz/check", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		assert.True(t, recordAttemptCalled, "UserService.RecordQuizAttempt should be called for authenticated user")
		assert.Equal(t, userID, recordedUserID)
		assert.Equal(t, commonCheckAnswerRequest.QuizID, recordedQuizID)
		assert.Equal(t, commonCheckAnswerRequest.UserAnswer, recordedUserAnswer)
	})

	t.Run("Anonymous User", func(t *testing.T) {
		setup()
		anonCachePutCalled := false

		mockQuizSvc.CheckAnswerFunc = func(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error) {
			assert.Equal(t, commonCheckAnswerRequest.QuizID, req.QuizID)
			assert.Equal(t, commonCheckAnswerRequest.UserAnswer, req.UserAnswer)
			return commonDomainResult, nil
		}
		// Ensure RecordQuizAttempt is not called
		mockUserSvc.RecordQuizAttemptFunc = func(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error {
			assert.Fail(t, "UserService.RecordQuizAttempt should not be called for anonymous user")
			return errors.New("RecordQuizAttempt should not be called")
		}
		mockAnonCacheSvc.PutFunc = func(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
			anonCachePutCalled = true
			assert.NotEmpty(t, requestID, "RequestID for cache should not be empty for anonymous user")
			assert.IsType(t, "string", requestID)
			assert.Equal(t, commonDomainResult, result)
			return nil
		}

		// Create a fiber app without setting UserIDKey
		app := fiber.New(fiber.Config{
			ErrorHandler: middleware.ErrorHandler(),
		})
		app.Post("/quiz/check", quizHandler.CheckAnswer)

		reqBodyBytes, _ := json.Marshal(commonCheckAnswerRequest)
		req := httptest.NewRequest("POST", "/quiz/check", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		assert.True(t, anonCachePutCalled, "AnonymousResultCacheService.Put should be called for anonymous user")
	})
}
