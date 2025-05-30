package middleware

import (
	"net/http"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/logger"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

// ErrorResponse represents the error response structure
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// mapErrorToHTTPStatus maps domain errors to HTTP status codes
func mapErrorToHTTPStatus(err *domain.DomainError) int {
	switch err.Code {
	case domain.ErrNotFound, domain.ErrQuizNotFound:
		return http.StatusNotFound
	case domain.ErrInvalidInput, domain.ErrInvalidAnswer, domain.ErrInvalidCategory:
		return http.StatusBadRequest
	case domain.ErrUnauthorized:
		return http.StatusUnauthorized
	case domain.ErrLLMServiceError:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ErrorHandler is a middleware that handles errors and returns appropriate HTTP responses
func ErrorHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Recover from panic
		defer func() {
			if err := recover(); err != nil {
				l := logger.Get()
				l.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Path()))

				appErr := domain.NewInternalError("Internal server error", nil)
				respondWithError(c, appErr)
			}
		}()

		// Process request
		err := c.Next()
		if err != nil {
			respondWithError(c, err)
		}

		return nil
	}
}

// respondWithError sends an error response to the client
func respondWithError(c fiber.Ctx, err error) {
	l := logger.Get()

	var appErr *domain.DomainError
	if e, ok := err.(*domain.DomainError); ok {
		appErr = e
	} else if e, ok := err.(*fiber.Error); ok {
		// Convert Fiber errors to AppError
		appErr = domain.NewInternalError(e.Message, nil)
	} else {
		appErr = domain.NewInternalError("Internal server error", err)
	}

	status := mapErrorToHTTPStatus(appErr)
	response := ErrorResponse{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Status:  status,
	}

	if err := c.Status(status).JSON(response); err != nil {
		l.Error("Failed to encode error response",
			zap.Error(err))
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to encode error response",
			"status":  fiber.StatusInternalServerError,
		})
	}
}
