package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt" // Added for fmt.Errorf
	"io"
	"net/http"
	"net/http/httptest"
	"quiz-byte/internal/domain"
	dto "quiz-byte/internal/dto"
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"testing"

	"log"                       // Added for logger init fatal
	"os"                        // Added for logger init
	"quiz-byte/internal/config" // Added for logger init
	"quiz-byte/internal/logger" // Added for logger init

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMain(m *testing.M) {
	// Initialize logger for handler tests
	// Note: Using a basic config. If specific config values are needed for logger in tests, adjust this.
	// The logger's behavior (JSON vs console) often depends on ENV var, which might be set globally for tests.
	cfg := &config.Config{
		// Populate with minimal fields if logger.Initialize depends on them,
		// otherwise, a zero-valued struct might be okay if ENV="test" is the main driver.
		// Example: Env: "test", might be read by logger.Initialize internally if it uses cfg.Env
	}
	if err := logger.Initialize(cfg.Logger); err != nil {
		log.Fatalf("Failed to initialize logger for handler tests: %v", err)
	}
	defer func() {
		if logger.Get() != nil {
			_ = logger.Sync()
		}
	}()

	// Run tests
	exitCode := m.Run()
	os.Exit(exitCode)
}

// MockUserService is a mock implementation of service.UserService
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) CreateQuizAttempt(userID, quizID, userAnswer string) error {
	args := m.Called(userID, quizID, userAnswer)
	return args.Error(0)
}

func (m *MockUserService) CreateUser(user *domain.User) (*domain.User, error) {
	args := m.Called(user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) GetAllUsers() ([]*domain.User, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserService) GetUserByID(id string) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) UpdateUser(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserService) DeleteUser(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

// GetUserProfile retrieves a user's profile.
func (m *MockUserService) GetUserProfile(ctx context.Context, userID string) (*dto.UserProfileResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UserProfileResponse), args.Error(1)
}

// GetUserQuizAttempts retrieves user's quiz attempts.
func (m *MockUserService) GetUserQuizAttempts(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserQuizAttemptsResponse, error) {
	args := m.Called(ctx, userID, filters, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UserQuizAttemptsResponse), args.Error(1)
}

// GetUserIncorrectAnswers retrieves user's incorrect answers.
func (m *MockUserService) GetUserIncorrectAnswers(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserIncorrectAnswersResponse, error) {
	args := m.Called(ctx, userID, filters, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UserIncorrectAnswersResponse), args.Error(1)
}

// RecordQuizAttempt records a user's quiz attempt.
func (m *MockUserService) RecordQuizAttempt(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error {
	args := m.Called(ctx, userID, quizID, userAnswer, evalResult)
	return args.Error(0)
}

// GetUserRecommendations retrieves user's quiz recommendations.
func (m *MockUserService) GetUserRecommendations(ctx context.Context, userID string, limit int, optionalSubCategoryID string) (*dto.QuizRecommendationsResponse, error) {
	args := m.Called(ctx, userID, limit, optionalSubCategoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.QuizRecommendationsResponse), args.Error(1)
}

// MockQuizRepository is a mock object that implements the QuizRepository interface.
type MockQuizRepository struct {
	mock.Mock
}

func (m *MockQuizRepository) GetAllSubCategories() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockQuizRepository) GetRandomQuizBySubCategory(subCategory string) (*models.Quiz, error) {
	args := m.Called(subCategory)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetQuizByID(id int) (*models.Quiz, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Quiz), args.Error(1)
}

func (m *MockQuizRepository) GetQuizzesByCriteria(limit int, offset int, criteria map[string]interface{}) ([]*models.Quiz, error) {
	args := m.Called(limit, offset, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Quiz), args.Error(1)
}

// MockQuizService is a mock implementation of service.QuizService
type MockQuizService struct {
	mock.Mock
}

func (m *MockQuizService) GetRandomQuiz(subCategory string) (*dto.QuizResponse, error) {
	args := m.Called(subCategory)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.QuizResponse), args.Error(1)
}

func (m *MockQuizService) CheckAnswer(req *dto.CheckAnswerRequest) (*dto.CheckAnswerResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.CheckAnswerResponse), args.Error(1)
}

func (m *MockQuizService) GetAllSubCategories() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockQuizService) GetBulkQuizzes(req *dto.BulkQuizzesRequest) (*dto.BulkQuizzesResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.BulkQuizzesResponse), args.Error(1)
}

func TestGetAllSubCategories(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   []string
		mockError      error
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "Success",
			mockResponse:   []string{"math", "science", "history"},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"sub_category_ids": []interface{}{"math", "science", "history"},
			},
		},
		{
			name:           "Internal Error",
			mockResponse:   nil,
			mockError:      fmt.Errorf("a very direct error"), // Simplified error for testing mock behavior
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve subcategory IDs",
				"status":  float64(500), // JSON unmarshals numbers as float64
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{
				ErrorHandler: middleware.ErrorHandler(),
			})
			mockQuizService := new(MockQuizService)
			mockUserService := new(MockUserService)
			handler := NewQuizHandler(mockQuizService, mockUserService, &MockAnonymousResultCacheService{})
			app.Get("/quiz/categories", handler.GetAllSubCategories)

			// Setup mock
			mockQuizService.On("GetAllSubCategories").Return(tt.mockResponse, tt.mockError).Once() // Added .Once() for clarity

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/quiz/categories", nil)
			resp, _ := app.Test(req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Only try to parse JSON body if it's not an error case, or specifically handle error JSON format
			if tt.expectedStatus == http.StatusOK {
				var body map[string]interface{}
				err := json.NewDecoder(resp.Body).Decode(&body)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, body)
			} else {
				// For error cases, we need to read the raw body instead of trying to parse it directly
				bodyBytes, _ := io.ReadAll(resp.Body)
				if len(bodyBytes) > 0 {
					var errorBody map[string]interface{}
					err := json.Unmarshal(bodyBytes, &errorBody)
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedBody, errorBody)
				}
			}
		})
	}
}

func TestGetRandomQuiz(t *testing.T) {
	// Setup
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler(),
	})
	mockQuizService := new(MockQuizService)
	mockUserService := new(MockUserService)
	handler := NewQuizHandler(mockQuizService, mockUserService, &MockAnonymousResultCacheService{})

	app.Get("/quiz/random/:subCategory", handler.GetRandomQuiz)

	tests := []struct {
		name           string
		subCategory    string
		mockResponse   *dto.QuizResponse
		mockError      error
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:        "Success",
			subCategory: "math",
			mockResponse: &dto.QuizResponse{ // This is what the service returns
				ID:           "test-quiz-id-123", // Use a fixed ID for assertion
				Question:     "What is 2+2?",
				ModelAnswers: []string{"4", "four"},
				Keywords:     []string{"arithmetic", "addition", "easy"},
				DiffLevel:    "easy", // Service should populate this
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{ // This should match the fields of dto.QuizResponse
				"id":            "test-quiz-id-123",
				"question":      "What is 2+2?",
				"model_answers": []interface{}{"4", "four"}, // Note: json unmarshals to []interface{}
				"keywords":      []interface{}{"arithmetic", "addition", "easy"},
				"diff_level":    "easy",
			},
		},
		{
			name:           "Invalid Category",
			subCategory:    "invalid",
			mockResponse:   nil,
			mockError:      domain.NewInvalidCategoryError("invalid"), // Use the specific error type
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"code":    "INVALID_CATEGORY",
				"message": "Invalid category: invalid",
				"status":  float64(400), // JSON에서 숫자는 float64로 디코딩됩니다.
				"details": map[string]interface{}{
					"category": "invalid",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockQuizService.On("GetRandomQuiz", tt.subCategory).Return(tt.mockResponse, tt.mockError)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/quiz/random/"+tt.subCategory, nil)
			resp, _ := app.Test(req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Only try to parse JSON body if it's not an error case, or specifically handle error JSON format
			if tt.expectedStatus == http.StatusOK {
				var body map[string]interface{}
				err := json.NewDecoder(resp.Body).Decode(&body)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, body)
			} else {
				// For error cases, we need to read the raw body instead of trying to parse it directly
				bodyBytes, _ := io.ReadAll(resp.Body)
				if len(bodyBytes) > 0 {
					var errorBody map[string]interface{}
					err := json.Unmarshal(bodyBytes, &errorBody)
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedBody, errorBody)
				}
			}
		})
	}
}

func TestCheckAnswer(t *testing.T) {
	// Setup
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler(),
	})
	mockQuizService := new(MockQuizService)
	mockUserService := new(MockUserService)
	mockCacheService := new(MockAnonymousResultCacheService)
	handler := NewQuizHandler(mockQuizService, mockUserService, mockCacheService)

	app.Post("/quiz/check", handler.CheckAnswer)

	tests := []struct {
		name           string
		requestBody    *dto.CheckAnswerRequest
		mockResponse   *dto.CheckAnswerResponse
		mockError      error
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "Success",
			requestBody: &dto.CheckAnswerRequest{
				QuizID:     util.NewULID(),
				UserAnswer: "4",
			},
			mockResponse: &dto.CheckAnswerResponse{
				Score:          1.0,
				Explanation:    "Correct! 2+2=4",
				KeywordMatches: []string{"4"},
				Completeness:   1.0,
				Relevance:      1.0,
				Accuracy:       1.0,
				ModelAnswer:    "4",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{ // Should match dto.CheckAnswerResponse
				"score":           1.0,
				"explanation":     "Correct! 2+2=4",
				"keyword_matches": []interface{}{"4"}, // json unmarshals to []interface{}
				"completeness":    1.0,
				"relevance":       1.0,
				"accuracy":        1.0,
				"model_answer":    "4",
			},
		},
		{
			name: "Quiz Not Found",
			requestBody: &dto.CheckAnswerRequest{
				QuizID:     util.NewULID(), // Will be different each run, but that's okay for mock
				UserAnswer: "4",
			},
			mockResponse:   nil,
			mockError:      domain.NewQuizNotFoundError("999"), // Use specific error type
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{ // Handler returns ErrorResponse structure
				"code":    "QUIZ_NOT_FOUND",
				"message": "Quiz not found with ID: 999",
				"status":  float64(404), // JSON unmarshals numbers as float64
				"details": map[string]interface{}{
					"quiz_id": "999",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks for each test case
			mockQuizService.ExpectedCalls = nil
			mockCacheService.ExpectedCalls = nil

			// Setup mock for quiz service
			mockQuizService.On("CheckAnswer", tt.requestBody).Return(tt.mockResponse, tt.mockError)

			// Setup mock for cache service (for anonymous users when successful)
			// The handler calls Put when user is not authenticated and no error occurs
			if tt.mockError == nil && tt.mockResponse != nil {
				mockCacheService.On("Put", mock.Anything, mock.AnythingOfType("string"), tt.mockResponse).Return(nil)
			}

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/quiz/check", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Parse response body
			bodyBytes, _ := io.ReadAll(resp.Body)
			if len(bodyBytes) > 0 {
				var responseBody map[string]interface{}
				err := json.Unmarshal(bodyBytes, &responseBody)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, responseBody)
			}
		})
	}
}

// MockAnonymousResultCacheService is a mock implementation of service.AnonymousResultCacheService
type MockAnonymousResultCacheService struct {
	mock.Mock
}

func (m *MockAnonymousResultCacheService) Put(ctx context.Context, requestID string, result *dto.CheckAnswerResponse) error {
	args := m.Called(ctx, requestID, result)
	return args.Error(0)
}

func (m *MockAnonymousResultCacheService) Get(ctx context.Context, requestID string) (*dto.CheckAnswerResponse, error) {
	args := m.Called(ctx, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.CheckAnswerResponse), args.Error(1)
}
