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

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func TestGetAllSubCategories(t *testing.T) {
	// Setup
	app := fiber.New()
	mockService := new(MockQuizService)
	handler := NewQuizHandler(mockService)

	app.Get("/quiz/categories", handler.GetAllSubCategories)

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
				"categories": []interface{}{"math", "science", "history"},
			},
		},
		{
			name:           "Internal Error",
			mockResponse:   nil,
			mockError:      domain.NewValidationError("database error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"error": "database error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockService.On("GetAllSubCategories").Return(tt.mockResponse, tt.mockError)

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
			mockResponse: &dto.QuizResponse{
				ID:           util.NewULID(),
				Question:     "What is 2+2?",
				ModelAnswers: []string{"3", "4", "5"},
				Keywords:     []string{"addition", "math"},
				DiffLevel:    "math",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"id":             float64(1),
				"question_text":  "What is 2+2?",
				"answer_options": []interface{}{"3", "4", "5"},
				"difficulty":     float64(1),
				"sub_category":   "math",
			},
		},
		{
			name:           "Invalid Category",
			subCategory:    "invalid",
			mockResponse:   nil,
			mockError:      domain.NewValidationError("invalid"),
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error": "invalid category: invalid",
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
				NextQuizID:     util.NewULID(),
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"is_correct":     true,
				"score":          float64(1.0),
				"explanation":    "Correct! 2+2=4",
				"correct_answer": "4",
			},
		},
		{
			name: "Quiz Not Found",
			requestBody: &dto.CheckAnswerRequest{
				QuizID:     util.NewULID(),
				UserAnswer: "4",
			},
			mockResponse:   nil,
			mockError:      domain.NewValidationError("999"),
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"error": "quiz not found: 999",
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
