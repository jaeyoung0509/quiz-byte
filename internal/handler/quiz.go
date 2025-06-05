package handler

import (
	"context"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/service"
	"quiz-byte/internal/util" // Added for NewULID
	"quiz-byte/internal/validation"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const DefaultBulkQuizCount = 10

// QuizHandler handles quiz-related HTTP requests following Clean Architecture principles
type QuizHandler struct {
	quizService               service.QuizService
	userService               service.UserService
	anonymousResultCacheService service.AnonymousResultCacheService // Added
	validator                 *validation.Validator
}

// NewQuizHandler creates a new QuizHandler instance
func NewQuizHandler(
	quizService service.QuizService,
	userService service.UserService,
	anonymousResultCacheService service.AnonymousResultCacheService, // Added
) *QuizHandler {
	return &QuizHandler{
		quizService:               quizService,
		userService:               userService,
		anonymousResultCacheService: anonymousResultCacheService, // Added
		validator:                 validation.NewValidator(),
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
	_, err := h.quizService.GetAllSubCategories()
	if err != nil {
		logger.Get().Error("Failed to get categories", zap.Error(err))
		return domain.NewInternalError("Failed to retrieve categories", err)
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

	// Get validated sub_category from middleware or validate here
	subCategory, ok := c.Locals("validated_sub_category").(string)
	if !ok {
		// Fallback validation if middleware wasn't used
		subCategory = c.Params("subCategory")
		if subCategory == "" {
			subCategory = c.Query("sub_category")
		}

		if validationErrors := h.validator.ValidateSubCategory(subCategory); len(validationErrors) > 0 {
			return validationErrors
		}
	}

	if userID != "" {
		appLogger.Info("Random quiz requested by user", zap.String("userID", userID), zap.String("sub_category", subCategory))
	} else {
		appLogger.Info("Random quiz requested (unauthenticated)", zap.String("sub_category", subCategory))
	}

	quiz, err := h.quizService.GetRandomQuiz(subCategory)
	if err != nil {
		appLogger.Error("Failed to get random quiz from service",
			zap.Error(err),
			zap.String("sub_category", subCategory),
		)
		return err // Return error directly, middleware will handle it
	}

	return c.JSON(dto.QuizResponse{
		ID:           quiz.ID,
		Question:     quiz.Question,
		Keywords:     quiz.Keywords,
		ModelAnswers: quiz.ModelAnswers,
		DiffLevel:    quiz.DiffLevel,
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
// @Security ApiKeyAuth
func (h *QuizHandler) CheckAnswer(c *fiber.Ctx) error {
	appLogger := logger.Get()

	var req dto.CheckAnswerRequest
	if err := c.BodyParser(&req); err != nil {
		appLogger.Warn("Failed to parse request body for CheckAnswer", zap.Error(err))
		return domain.NewValidationError("Invalid request body format")
	}

	// Validate request using validator
	if validationErrors := h.validator.ValidateCheckAnswerRequest(req.QuizID, req.UserAnswer); len(validationErrors) > 0 {
		return validationErrors
	}

	// Call the service to check answer
	domainResult, err := h.quizService.CheckAnswer(&req)
	if err != nil {
		appLogger.Error("Failed to check answer via QuizService",
			zap.Error(err),
			zap.String("quiz_id", req.QuizID),
		)
		return err // Return error directly, middleware will handle it
	}

	userID, userIsAuthenticated := c.Locals(middleware.UserIDKey).(string)
	if userIsAuthenticated && userID != "" {
		// Authenticated user: Record quiz attempt
		domainAnswerForRecord := &domain.Answer{
			Score:          domainResult.Score,
			Explanation:    domainResult.Explanation,
			KeywordMatches: domainResult.KeywordMatches,
			Completeness:   domainResult.Completeness,
			Relevance:      domainResult.Relevance,
			Accuracy:       domainResult.Accuracy,
			AnsweredAt:     time.Now(),
		}
		h.recordQuizAttemptAsync(c, req, domainAnswerForRecord)
	} else {
		// Anonymous user: Cache the result
		if h.anonymousResultCacheService != nil {
			requestIDForCache := util.NewULID() // Generate a unique ID for caching this specific result

			errCache := h.anonymousResultCacheService.Put(c.Context(), requestIDForCache, domainResult)
			if errCache != nil {
				appLogger.Error("Failed to cache anonymous user quiz result",
					zap.Error(errCache),
					zap.String("quizID", req.QuizID),
					zap.String("requestIDForCache", requestIDForCache),
				)
				// Do not fail the request, just log the caching error.
			} else {
				appLogger.Info("Anonymous user quiz result cached",
					zap.String("requestIDForCache", requestIDForCache),
					zap.String("quizID", req.QuizID),
				)
			}
		} else {
			appLogger.Warn("AnonymousResultCacheService is nil. Cannot cache anonymous user quiz result.", zap.String("quizID", req.QuizID))
		}
		// For anonymous users, we already logged that the attempt won't be recorded in the previous subtask.
		// If specific logging about *not* recording is still desired here, it can be added.
		// The existing log from previous subtask:
		// appLogger.Info("CheckAnswer processing for anonymous user. Quiz attempt will not be recorded.", zap.String("quizID", req.QuizID))
		// This new logic focuses on caching.
	}

	return c.JSON(domainResult)
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

	// Get validated parameters from middleware or validate here
	subCategory, subCategoryOk := c.Locals("validated_sub_category").(string)
	count, countOk := c.Locals("validated_count").(int)

	if !subCategoryOk || !countOk {
		// Fallback validation if middleware wasn't used
		subCategory = c.Query("sub_category")
		count = DefaultBulkQuizCount

		if countStr := c.Query("count"); countStr != "" {
			var err error
			count, err = strconv.Atoi(countStr)
			if err != nil {
				return domain.ValidationErrors{
					domain.NewInvalidFormatError("count", countStr),
				}
			}
		}

		if validationErrors := h.validator.ValidateBulkQuizzesRequest(subCategory, count); len(validationErrors) > 0 {
			return validationErrors
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

	result, err := h.quizService.GetBulkQuizzes(reqDTO)
	if err != nil {
		appLogger.Error("Failed to get bulk quizzes from service",
			zap.Error(err),
			zap.String("sub_category", subCategory),
			zap.Int("count", count),
		)
		return err // Return error directly, middleware will handle it
	}

	return c.JSON(result)
}

// recordQuizAttemptAsync records quiz attempt in background to avoid blocking response
func (h *QuizHandler) recordQuizAttemptAsync(c *fiber.Ctx, req dto.CheckAnswerRequest, result *domain.Answer) {
	userID, ok := c.Locals(middleware.UserIDKey).(string)
	if !ok || userID == "" || h.userService == nil {
		return
	}

	appLogger := logger.Get()
	appLogger.Info("Quiz attempt recording initiated for user",
		zap.String("userID", userID),
		zap.String("quizID", req.QuizID))

	// Convert response to domain answer
	answer := &domain.Answer{
		Score:          result.Score,
		Explanation:    result.Explanation,
		KeywordMatches: result.KeywordMatches,
		AnsweredAt:     time.Now(),
		Completeness:   result.Completeness,
		Relevance:      result.Relevance,
		Accuracy:       result.Accuracy,
	}

	// Record attempt asynchronously
	go func(ctx context.Context, currentUserID, quizID, userAnswer string, evalResult *domain.Answer) {
		if err := h.userService.RecordQuizAttempt(ctx, currentUserID, quizID, userAnswer, evalResult); err != nil {
			logger.Get().Error("Failed to record user quiz attempt in goroutine",
				zap.String("userID", currentUserID),
				zap.String("quizID", quizID),
				zap.Error(err),
			)
		}
	}(c.Context(), userID, req.QuizID, req.UserAnswer, answer)
}
