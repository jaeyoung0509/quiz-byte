package handler

import (
	"errors"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/logger"     // Added
	"quiz-byte/internal/middleware" // For UserIDKey and ErrorResponse
	"quiz-byte/internal/service"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap" // Added

	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// GetMyProfile retrieves the profile of the currently authenticated user.
// @Summary Get My Profile
// @Description Retrieves the profile information of the logged-in user.
// @Tags users
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.UserProfileResponse
// @Failure 401 {object} middleware.ErrorResponse "Unauthorized"
// @Failure 404 {object} middleware.ErrorResponse "User not found"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /users/me [get]
func (h *UserHandler) GetMyProfile(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userID, ok := c.Locals(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		appLogger.Warn("User ID not found in context for GetMyProfile", zap.String("path", c.Path()))
		return c.Status(fiber.StatusUnauthorized).JSON(middleware.ErrorResponse{
			Code: "INVALID_USER_CONTEXT", Message: "User ID not found in context", Status: fiber.StatusUnauthorized,
		})
	}

	profile, err := h.userService.GetUserProfile(c.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserProfileNotFound) {
			appLogger.Info("User profile not found", zap.String("userID", userID), zap.Error(err))
			return c.Status(fiber.StatusNotFound).JSON(middleware.ErrorResponse{
				Code: "USER_PROFILE_NOT_FOUND", Message: err.Error(), Status: fiber.StatusNotFound,
			})
		}
		appLogger.Error("Failed to get user profile", zap.String("userID", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "GET_PROFILE_FAILED", Message: "Failed to retrieve user profile", Status: fiber.StatusInternalServerError,
		})
	}
	appLogger.Info("User profile retrieved", zap.String("userID", userID))
	return c.JSON(profile)
}

func parsePagination(c *fiber.Ctx) dto.Pagination {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if limit <= 0 {
		limit = 10
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return dto.Pagination{Limit: limit, Offset: offset, Page: page}
}

func parseAttemptFilters(c *fiber.Ctx) dto.AttemptFilters {
	var isCorrectPtr *bool
	isCorrectQuery := c.Query("is_correct")
	if isCorrectQuery != "" {
		isCorrectVal, err := strconv.ParseBool(isCorrectQuery)
		if err == nil {
			isCorrectPtr = &isCorrectVal
		}
	}

	// Attempt to parse StartDate
	var startDate time.Time
	startDateStr := c.Query("start_date")
	if startDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsedDate
		}
		// Optionally log error if date parsing fails, or handle as bad request
	}

	// Attempt to parse EndDate
	var endDate time.Time
	endDateStr := c.Query("end_date")
	if endDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsedDate
		}
	}

	// TODO: fix it
	_ = startDate
	_ = endDate

	return dto.AttemptFilters{
		CategoryID: c.Query("category_id"),
		StartDate:  startDateStr, // Keep as string for now, service layer will parse
		EndDate:    endDateStr,   // Keep as string for now, service layer will parse
		IsCorrect:  isCorrectPtr,
		SortBy:     c.Query("sort_by", "attempted_at"),             // Default sort by time
		SortOrder:  strings.ToUpper(c.Query("sort_order", "DESC")), // Default sort order DESC
	}
}

// GetMyAttempts retrieves the quiz attempt history of the authenticated user.
// @Summary Get My Quiz Attempts
// @Description Retrieves a paginated list of the logged-in user's quiz attempts, with filtering options.
// @Tags users
// @Security ApiKeyAuth
// @Produce json
// @Param limit query int false "Number of items per page (default 10)"
// @Param page query int false "Page number (default 1)"
// @Param category_id query string false "Filter by category ID"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD)"
// @Param is_correct query bool false "Filter by correctness (true/false)"
// @Param sort_by query string false "Sort by field (e.g., 'attempted_at', 'score', default 'attempted_at')"
// @Param sort_order query string false "Sort order ('ASC', 'DESC', default 'DESC')"
// @Success 200 {object} dto.UserQuizAttemptsResponse
// @Failure 401 {object} middleware.ErrorResponse "Unauthorized"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /users/me/attempts [get]
func (h *UserHandler) GetMyAttempts(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userID, ok := c.Locals(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		appLogger.Warn("User ID not found in context for GetMyAttempts", zap.String("path", c.Path()))
		return c.Status(fiber.StatusUnauthorized).JSON(middleware.ErrorResponse{
			Code: "INVALID_USER_CONTEXT", Message: "User ID not found in context", Status: fiber.StatusUnauthorized,
		})
	}

	pagination := parsePagination(c)
	filters := parseAttemptFilters(c)
	// Validate SortOrder
	if filters.SortOrder != "ASC" && filters.SortOrder != "DESC" {
		filters.SortOrder = "DESC"
	}

	appLogger.Info("User quiz attempts requested",
		zap.String("userID", userID),
		zap.Any("filters", filters),
		zap.Any("pagination", pagination))

	response, err := h.userService.GetUserQuizAttempts(c.Context(), userID, filters, pagination)
	if err != nil {
		appLogger.Error("Failed to get user attempts", zap.String("userID", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "GET_ATTEMPTS_FAILED", Message: "Failed to retrieve quiz attempts", Status: fiber.StatusInternalServerError,
		})
	}
	return c.JSON(response)
}

// GetMyIncorrectAnswers retrieves the incorrect answers of the authenticated user.
// @Summary Get My Incorrect Answers
// @Description Retrieves a paginated list of the logged-in user's incorrect quiz answers, with filtering options.
// @Tags users
// @Security ApiKeyAuth
// @Produce json
// @Param limit query int false "Number of items per page (default 10)"
// @Param page query int false "Page number (default 1)"
// @Param category_id query string false "Filter by category ID"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD)"
// @Param sort_by query string false "Sort by field (e.g., 'attempted_at', 'score', default 'attempted_at')"
// @Param sort_order query string false "Sort order ('ASC', 'DESC', default 'DESC')"
// @Success 200 {object} dto.UserIncorrectAnswersResponse
// @Failure 401 {object} middleware.ErrorResponse "Unauthorized"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /users/me/incorrect-answers [get]
func (h *UserHandler) GetMyIncorrectAnswers(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userID, ok := c.Locals(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		appLogger.Warn("User ID not found in context for GetMyIncorrectAnswers", zap.String("path", c.Path()))
		return c.Status(fiber.StatusUnauthorized).JSON(middleware.ErrorResponse{
			Code: "INVALID_USER_CONTEXT", Message: "User ID not found in context", Status: fiber.StatusUnauthorized,
		})
	}

	pagination := parsePagination(c)
	filters := parseAttemptFilters(c)
	if filters.SortOrder != "ASC" && filters.SortOrder != "DESC" {
		filters.SortOrder = "DESC"
	}

	appLogger.Info("User incorrect answers requested",
		zap.String("userID", userID),
		zap.Any("filters", filters),
		zap.Any("pagination", pagination))

	response, err := h.userService.GetUserIncorrectAnswers(c.Context(), userID, filters, pagination)
	if err != nil {
		appLogger.Error("Failed to get user incorrect answers", zap.String("userID", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "GET_INCORRECT_ANSWERS_FAILED", Message: "Failed to retrieve incorrect answers", Status: fiber.StatusInternalServerError,
		})
	}
	return c.JSON(response)
}

// GetMyRecommendations retrieves personalized quiz recommendations for the authenticated user.
// @Summary Get My Quiz Recommendations
// @Description Retrieves a list of personalized quiz recommendations for the logged-in user.
// @Tags users
// @Security ApiKeyAuth
// @Produce json
// @Param limit query int false "Number of recommendations to fetch (default 10)"
// @Param sub_category_id query string false "Optional: Filter recommendations by sub-category ID"
// @Success 200 {object} dto.QuizRecommendationsResponse
// @Failure 401 {object} middleware.ErrorResponse "Unauthorized"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /users/me/recommendations [get]
func (h *UserHandler) GetMyRecommendations(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userID, ok := c.Locals(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		appLogger.Warn("User ID not found in context for GetMyRecommendations", zap.String("path", c.Path()))
		return c.Status(fiber.StatusUnauthorized).JSON(middleware.ErrorResponse{
			Code: "INVALID_USER_CONTEXT", Message: "User ID not found in context", Status: fiber.StatusUnauthorized,
		})
	}

	limit, err := strconv.Atoi(c.Query("limit", "10"))
	if err != nil || limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 50 { // Max limit
		limit = 50
	}

	optionalSubCategoryID := c.Query("sub_category_id")

	appLogger.Info("User recommendations requested",
		zap.String("userID", userID),
		zap.Int("limit", limit),
		zap.String("sub_category_id", optionalSubCategoryID))

	recommendations, err := h.userService.GetUserRecommendations(c.Context(), userID, limit, optionalSubCategoryID)
	if err != nil {
		appLogger.Error("Failed to get user recommendations",
			zap.String("userID", userID),
			zap.Int("limit", limit),
			zap.String("sub_category_id", optionalSubCategoryID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "GET_RECOMMENDATIONS_FAILED", Message: "Failed to retrieve recommendations", Status: fiber.StatusInternalServerError,
		})
	}

	return c.JSON(recommendations)
}
