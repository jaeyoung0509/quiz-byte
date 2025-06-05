package handler

import (
	"context"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/middleware" // Added for UserIDKey
	"quiz-byte/internal/service"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const DefaultBulkQuizCount = 10

// QuizHandler handles quiz-related HTTP requests
type QuizHandler struct {
	service service.QuizService
	// Add UserService to record attempts.
	// This is a common pattern, though sometimes a dedicated "AttemptService" might be used.
	// Or, QuizService itself could be made aware of UserService or have RecordAttempt method.
	// For simplicity here, let's assume QuizHandler gets UserService injected.
	// This means NewQuizHandler needs to be updated.
	userService service.UserService
}

// NewQuizHandler creates a new QuizHandler instance
func NewQuizHandler(quizService service.QuizService, userService service.UserService) *QuizHandler {
	return &QuizHandler{
		service:     quizService,
		userService: userService,
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
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{ // Changed to middleware.ErrorResponse
			Code: "INTERNAL_ERROR", Message: "Failed to retrieve categories", Status: fiber.StatusInternalServerError,
		})
	}

	return c.JSON(dto.CategoryResponse{
		ID:          "",
		Name:        "All Categories",
		Description: "List of all quiz categories",
	})
}

// GetRandomQuiz godoc
// @Summary Get a random quiz
// @Description Get a random quiz by sub category. Requires authentication.
// @Tags quiz
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param sub_category query string true "Sub Category"
// @Success 200 {object} dto.QuizResponse
// @Failure 400 {object} middleware.ErrorResponse "Invalid request (e.g., missing sub_category)"
// @Failure 401 {object} middleware.ErrorResponse "Unauthorized"
// @Failure 404 {object} middleware.ErrorResponse "Quiz or category not found"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /quiz [get]
func (h *QuizHandler) GetRandomQuiz(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userID, _ := c.Locals(middleware.UserIDKey).(string)
	subCategory := c.Params("subCategory") // Correctly read path parameter
	if subCategory == "" {
		// This check might be redundant if the route requires the param,
		// but can stay as a safeguard or be removed if route guarantees presence.
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
			Error: "INVALID_REQUEST",
		})
	}

	if userID != "" {
		appLogger.Info("Random quiz requested by user", zap.String("userID", userID), zap.String("sub_category", subCategory))
	} else {
		appLogger.Info("Random quiz requested (unauthenticated)", zap.String("sub_category", subCategory))
	}

	quiz, err := h.service.GetRandomQuiz(subCategory)
	if err != nil {
		appLogger.Error("Failed to get random quiz from service",
			zap.Error(err),
			zap.String("sub_category", subCategory),
		)

		// Using mapDomainErrorToHTTPStatus helper for consistency
		if domainErr, ok := err.(*domain.DomainError); ok {
			return c.Status(mapDomainErrorToHTTPStatus(domainErr)).JSON(middleware.ErrorResponse{
				Code: string(domainErr.Code), Message: domainErr.Message, Status: mapDomainErrorToHTTPStatus(domainErr),
			})
		}
		// Fallback for non-domain errors
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "INTERNAL_ERROR", Message: "Failed to retrieve random quiz", Status: fiber.StatusInternalServerError,
		})
	}

	return c.JSON(dto.QuizResponse{
		ID:           quiz.ID,
		Question:     quiz.Question,
		Keywords:     quiz.Keywords,
		ModelAnswers: quiz.ModelAnswers,
		DiffLevel:    quiz.DiffLevel, // Assuming quiz.DiffLevel is the string "easy", "medium", "hard"
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
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Failure 503 {object} middleware.ErrorResponse "LLM service unavailable"
// @Router /quiz/check [post]
// @Security ApiKeyAuth
func (h *QuizHandler) CheckAnswer(c *fiber.Ctx) error {
	appLogger := logger.Get()
	var req dto.CheckAnswerRequest
	if err := c.BodyParser(&req); err != nil {
		appLogger.Warn("Failed to parse request body for CheckAnswer", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "INVALID_REQUEST_BODY", Message: "Invalid request body", Status: fiber.StatusBadRequest,
		})
	}

	// Validate request
	if req.QuizID == "" {
		appLogger.Warn("QuizID missing in CheckAnswer request")
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "QUIZ_ID_REQUIRED", Message: "quiz_id is required", Status: fiber.StatusBadRequest,
		})
	}
	if req.UserAnswer == "" {
		appLogger.Warn("UserAnswer missing in CheckAnswer request", zap.String("quizID", req.QuizID))
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "ANSWER_REQUIRED", Message: "answer is required", Status: fiber.StatusBadRequest,
		})
	}

	// Call the service to get both the domain.Answer and the response DTO
	domainResult, err := h.service.CheckAnswer(&req) // Adjust service to return both if needed
	if err != nil {
		appLogger.Error("Failed to check answer via QuizService",
			zap.Error(err),
			zap.String("quiz_id", req.QuizID),
		)
		if domainErr, ok := err.(*domain.DomainError); ok {
			return c.Status(mapDomainErrorToHTTPStatus(domainErr)).JSON(middleware.ErrorResponse{
				Code: string(domainErr.Code), Message: domainErr.Message, Status: mapDomainErrorToHTTPStatus(domainErr),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "INTERNAL_ERROR", Message: "Error checking answer", Status: fiber.StatusInternalServerError,
		})
	}

	// Attempt to record the attempt if user is authenticated
	userID, ok := c.Locals(middleware.UserIDKey).(string)
	if ok && userID != "" && h.userService != nil {
		appLogger.Info("Quiz attempt recording initiated for user", zap.String("userID", userID), zap.String("quizID", req.QuizID))

		// Convert dto.CheckAnswerResponse to domain.Answer
		answer := &domain.Answer{
			Score:          domainResult.Score,
			Explanation:    domainResult.Explanation,
			KeywordMatches: domainResult.KeywordMatches,
			AnsweredAt:     time.Now(),
			Completeness:   domainResult.Completeness,
			Relevance:      domainResult.Relevance,
			Accuracy:       domainResult.Accuracy,
		}

		go func(ctx context.Context, currentUserID string, quizIDFromReq string, userAnswerFromReq string, evalResultFromService *domain.Answer) {
			errRecord := h.userService.RecordQuizAttempt(ctx, currentUserID, quizIDFromReq, userAnswerFromReq, evalResultFromService)
			if errRecord != nil {
				logger.Get().Error("Failed to record user quiz attempt in goroutine",
					zap.String("userID", currentUserID),
					zap.String("quizID", quizIDFromReq),
					zap.Error(errRecord),
				)
			}
		}(c.Context(), userID, req.QuizID, req.UserAnswer, answer)
	}

	if err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			switch domainErr.Code {
			case domain.ErrQuizNotFound:
				return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{
					Error: string(domainErr.Code),
				})
			case domain.ErrLLMServiceError:
				return c.Status(fiber.StatusServiceUnavailable).JSON(dto.ErrorResponse{
					Error: string(domainErr.Code),
				})
			default:
				return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
					Error: string(domain.ErrInternal),
				})
			}
		}
		// Fallback for non-DomainError types
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
			Error: string(domain.ErrInternal),
		})
	}

	return c.JSON(domainResult)
}

// Helper function (can be moved to a shared place or be part of existing error handling)
func mapDomainErrorToHTTPStatus(err *domain.DomainError) int {
	// This function might already exist or be similar to one in middleware/error.go
	switch err.Code {
	case domain.ErrQuizNotFound:
		return fiber.StatusNotFound
	case domain.ErrLLMServiceError:
		return fiber.StatusServiceUnavailable
	default:
		return fiber.StatusInternalServerError
	}
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
// @Failure 400 {object} middleware.ErrorResponse "Invalid request (e.g., missing sub_category or invalid count)"
// @Failure 401 {object} middleware.ErrorResponse "Unauthorized"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /quizzes [get]
// @Security ApiKeyAuth
func (h *QuizHandler) GetBulkQuizzes(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userID, _ := c.Locals(middleware.UserIDKey).(string)

	subCategory := c.Query("sub_category")
	if subCategory == "" {
		appLogger.Warn("sub_category query parameter missing for GetBulkQuizzes", zap.String("userID", userID))
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "INVALID_REQUEST", Message: "sub_category query parameter is required", Status: fiber.StatusBadRequest,
		})
	}

	countStr := c.Query("count")
	count := DefaultBulkQuizCount
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 || count > 50 {
			appLogger.Warn("Invalid count parameter for GetBulkQuizzes",
				zap.String("userID", userID),
				zap.String("count_str", countStr),
				zap.Error(err))
			return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
				Code: "INVALID_COUNT_PARAM", Message: "count must be a positive integer between 1 and 50", Status: fiber.StatusBadRequest,
			})
		}
	}

	if userID != "" {
		appLogger.Info("Bulk quizzes requested by user",
			zap.String("userID", userID),
			zap.String("sub_category", subCategory),
			zap.Int("count", count))
	} else {
		appLogger.Info("Bulk quizzes requested (unauthenticated)",
			zap.String("sub_category", subCategory),
			zap.Int("count", count))
	}

	reqDTO := &dto.BulkQuizzesRequest{
		SubCategory: subCategory,
		Count:       count,
	}

	result, err := h.service.GetBulkQuizzes(reqDTO)
	if err != nil {
		appLogger.Error("Failed to get bulk quizzes from service",
			zap.Error(err),
			zap.String("sub_category", subCategory),
			zap.Int("count", count),
		)
		if domainErr, ok := err.(*domain.DomainError); ok {
			return c.Status(mapDomainErrorToHTTPStatus(domainErr)).JSON(middleware.ErrorResponse{
				Code: string(domainErr.Code), Message: domainErr.Message, Status: mapDomainErrorToHTTPStatus(domainErr),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "INTERNAL_ERROR", Message: "Error getting bulk quizzes", Status: fiber.StatusInternalServerError,
		})
	}

	return c.JSON(result)
}
