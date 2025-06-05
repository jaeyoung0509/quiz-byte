package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quiz-byte/internal/domain"
	dto "quiz-byte/internal/dto"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"testing"
	"fmt" // Added for fmt.Errorf

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"quiz-byte/internal/config" // Added for logger init
	"quiz-byte/internal/logger" // Added for logger init
	"os"                        // Added for logger init
	"log"                       // Added for logger init fatal
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
	if err := logger.Initialize(cfg); err != nil {
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
				"id":          "", // Corrected to lowercase
				"name":        "All Categories", // Corrected to lowercase
				"description": "List of all quiz categories", // Corrected to lowercase
			},
		},
		{
			name:           "Internal Error",
			mockResponse:   nil,
			mockError:      fmt.Errorf("a very direct error"), // Simplified error for testing mock behavior
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{ // Handler returns a generic error message
				"error": "INTERNAL_ERROR", // Corrected to lowercase "error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockQuizService)
			handler := NewQuizHandler(mockService)
			app.Get("/quiz/categories", handler.GetAllSubCategories)

			// Setup mock
			mockService.On("GetAllSubCategories").Return(tt.mockResponse, tt.mockError).Once() // Added .Once() for clarity

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/quiz/categories", nil)
			resp, _ := app.Test(req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var body map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&body)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, body)
		})
	}
}

func TestGetRandomQuiz(t *testing.T) {
	// Setup
	app := fiber.New()
	mockService := new(MockQuizService)
	handler := NewQuizHandler(mockService)

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
			expectedBody: map[string]interface{}{ // Handler returns specific DTO
				"error": "INVALID_CATEGORY", // Corrected to lowercase "error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockService.On("GetRandomQuiz", tt.subCategory).Return(tt.mockResponse, tt.mockError)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/quiz/random/"+tt.subCategory, nil)
			resp, _ := app.Test(req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var body map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&body)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, body)
		})
	}
}

func TestCheckAnswer(t *testing.T) {
	// Setup
	app := fiber.New()
	mockService := new(MockQuizService)
	handler := NewQuizHandler(mockService)

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
			expectedBody: map[string]interface{}{ // Handler returns specific DTO
				"error": "QUIZ_NOT_FOUND", // Corrected to lowercase "error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockService.On("CheckAnswer", tt.requestBody).Return(tt.mockResponse, tt.mockError)

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/quiz/check", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var responseBody map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&responseBody)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, responseBody)
		})
	}
}
