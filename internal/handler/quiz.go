package handler

import (
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/service"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const DefaultBulkQuizCount = 10

// QuizHandler handles quiz-related HTTP requests
type QuizHandler struct {
	service service.QuizService
}

// NewQuizHandler creates a new QuizHandler instance
func NewQuizHandler(service service.QuizService) *QuizHandler {
	return &QuizHandler{
		service: service,
	}
}

// GetAllSubCategories godoc
// @Summary Get all quiz categories
// @Description Returns all available quiz categories
// @Tags categories
// @Accept json
// @Produce json
// @Success 200 {object} dto.CategoryResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /categories [get]
func (h *QuizHandler) GetAllSubCategories(c *fiber.Ctx) error {
	_, err := h.service.GetAllSubCategories()
	if err != nil {
		logger.Get().Error("Failed to get categories", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
			Error: "INTERNAL_ERROR",
		})
	}

	return c.JSON(dto.CategoryResponse{
		ID:          "", // No specific ID for all categories, string type
		Name:        "All Categories",
		Description: "List of all quiz categories",
	})
}

// GetRandomQuiz godoc
// @Summary Get a random quiz
// @Description Get a random quiz by sub category
// @Tags quiz
// @Accept json
// @Produce json
// @Param sub_category query string true "Sub Category"
// @Success 200 {object} dto.QuizResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /quiz [get]
// GetRandomQuiz handles GET /api/quiz
// @Summary Get a random quiz
// @Description Returns a random quiz question
// @Tags quiz
// @Accept json
// @Produce json
// @Success 200 {object} dto.QuizResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /quiz [get]
func (h *QuizHandler) GetRandomQuiz(c *fiber.Ctx) error {
	subCategory := c.Query("sub_category")
	if subCategory == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "INVALID_REQUEST",
		})
	}

	quiz, err := h.service.GetRandomQuiz(subCategory)
	if err != nil {
		logger.Get().Error("Failed to get random quiz",
			zap.Error(err),
			zap.String("sub_category", subCategory),
		)

		switch err.(type) {
		case *domain.InvalidCategoryError:
			return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
				Error: "INVALID_CATEGORY",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
				Error: "INTERNAL_ERROR",
			})
		}
	}

	return c.JSON(dto.QuizResponse{
		ID:           quiz.ID,
		Question:     quiz.Question,
		Keywords:     quiz.Keywords,
		ModelAnswers: quiz.ModelAnswers,
	})
}

// CheckAnswer godoc
// @Summary Check an answer for a quiz
// @Description Check an answer for a quiz
// @Tags quiz
// @Accept json
// @Produce json
// @Param answer body dto.CheckAnswerRequest true "Answer Request"
// @Success 200 {object} domain.Answer
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Failure 503 {object} dto.ErrorResponse
// @Router /quiz/check [post]
// CheckAnswer handles POST /api/quiz/check
// @Summary Check quiz answer
// @Description Checks if the provided answer is correct
// @Tags quiz
// @Accept json
// @Produce json
// @Param request body dto.CheckAnswerRequest true "Answer details"
// @Success 200 {object} dto.CheckAnswerResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /quiz/check [post]
func (h *QuizHandler) CheckAnswer(c *fiber.Ctx) error {
	var req dto.CheckAnswerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "INVALID_REQUEST",
		})
	}

	// Validate request
	if req.QuizID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "quiz_id is required",
		})
	}
	if req.UserAnswer == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "answer is required",
		})
	}

	result, err := h.service.CheckAnswer(&req)
	if err != nil {
		logger.Get().Error("Failed to check answer",
			zap.Error(err),
			zap.String("quiz_id", req.QuizID), // Changed to zap.String
		)

		switch err.(type) {
		case *domain.QuizNotFoundError:
			return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{
				Error: "QUIZ_NOT_FOUND",
			})
		case *domain.LLMServiceError:
			return c.Status(fiber.StatusServiceUnavailable).JSON(dto.ErrorResponse{
				Error: "LLM_SERVICE_UNAVAILABLE",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
				Error: "INTERNAL_ERROR",
			})
		}
	}

	return c.JSON(result)
}

// GetBulkQuizzes godoc
// @Summary Get multiple quizzes by sub-category
// @Description Returns a list of quizzes based on sub-category and count
// @Tags quiz
// @Accept json
// @Produce json
// @Param sub_category query string true "Sub-category of the quizzes"
// @Param count query int false "Number of quizzes to fetch (default: 10, max: 50)"
// @Success 200 {object} dto.BulkQuizzesResponse
// @Failure 400 {object} dto.ErrorResponse "INVALID_REQUEST if sub_category is missing or count is invalid"
// @Failure 500 {object} dto.ErrorResponse "INTERNAL_ERROR for other server-side issues"
// @Router /quizzes [get]
func (h *QuizHandler) GetBulkQuizzes(c *fiber.Ctx) error {
	subCategory := c.Query("sub_category")
	if subCategory == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "INVALID_REQUEST: sub_category is required",
		})
	}

	countStr := c.Query("count")
	count := DefaultBulkQuizCount // Default value
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 || count > 50 { // Basic validation for count
			return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
				Error: "INVALID_REQUEST: count must be a positive integer between 1 and 50",
			})
		}
	}

	req := &dto.BulkQuizzesRequest{
		SubCategory: subCategory,
		Count:       count,
	}

	result, err := h.service.GetBulkQuizzes(req)
	if err != nil {
		logger.Get().Error("Failed to get bulk quizzes",
			zap.Error(err),
			zap.String("sub_category", subCategory),
			zap.Int("count", count),
		)

		switch err.(type) {
		case *domain.InvalidCategoryError: // Assuming this error type might be returned by service
			return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
				Error: "INVALID_CATEGORY: " + err.Error(),
			})
		case *domain.InternalError: // Catch internal errors from service/repo
			return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
				Error: "INTERNAL_ERROR",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
				Error: "INTERNAL_ERROR",
			})
		}
	}

	return c.JSON(result)
}
