package handler

import (
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/service"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

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

// GetAllSubCategories handles GET /quiz/categories
// GetAllSubCategories handles GET /api/categories
func (h *QuizHandler) GetAllSubCategories(c fiber.Ctx) error {
	_, err := h.service.GetAllSubCategories()
	if err != nil {
		logger.Get().Error("Failed to get categories", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
			Error: "INTERNAL_ERROR",
		})
	}

	return c.JSON(dto.CategoryResponse{
		ID:          0, // No specific ID for all categories
		Name:        "All Categories",
		Description: "List of all quiz categories",
	})
}

// GetRandomQuiz handles GET /api/quiz
func (h *QuizHandler) GetRandomQuiz(c fiber.Ctx) error {
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

// CheckAnswer handles POST /api/quiz/check
func (h *QuizHandler) CheckAnswer(c fiber.Ctx) error {
	var req dto.AnswerRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "INVALID_REQUEST",
		})
	}

	// Validate request
	if req.QuizID == 0 {
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
			zap.Int64("quiz_id", req.QuizID),
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
