package middleware

import (
	"errors"
	"net/http"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/logger"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// ErrorResponse represents the standard error response structure
type ErrorResponse struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Status  int                    `json:"status"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ValidationErrorResponse represents validation error response
type ValidationErrorResponse struct {
	Code    string                   `json:"code"`
	Message string                   `json:"message"`
	Status  int                      `json:"status"`
	Errors  []domain.ValidationError `json:"errors"`
}

// ErrorHandler is a centralized error handling middleware
func ErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		logger := logger.Get()

		// Handle validation errors
		if validationErrs, ok := err.(domain.ValidationErrors); ok {
			logger.Warn("Validation errors occurred",
				zap.String("path", c.Path()),
				zap.Int("error_count", len(validationErrs)),
			)
			return c.Status(http.StatusBadRequest).JSON(ValidationErrorResponse{
				Code:    string(domain.CodeValidation),
				Message: "Request validation failed",
				Status:  http.StatusBadRequest,
				Errors:  validationErrs,
			})
		}

		// Handle domain errors
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapDomainErrorToHTTPStatus(domainErr)

			logger.Error("Domain error occurred",
				zap.String("code", string(domainErr.Code)),
				zap.String("message", domainErr.Message),
				zap.Int("status", statusCode),
				zap.Error(domainErr.Cause),
			)

			response := ErrorResponse{
				Code:    string(domainErr.Code),
				Message: domainErr.Message,
				Status:  statusCode,
			}

			if domainErr.Context != nil && len(domainErr.Context) > 0 {
				response.Details = domainErr.Context
			}

			return c.Status(statusCode).JSON(response)
		}

		// Handle fiber errors
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			logger.Warn("Fiber error occurred",
				zap.Int("code", fiberErr.Code),
				zap.String("message", fiberErr.Message),
			)
			return c.Status(fiberErr.Code).JSON(ErrorResponse{
				Code:    "HTTP_ERROR",
				Message: fiberErr.Message,
				Status:  fiberErr.Code,
			})
		}

		// Handle unknown errors
		logger.Error("Unknown error occurred",
			zap.String("path", c.Path()),
			zap.Error(err),
		)

		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{
			Code:    string(domain.CodeInternal),
			Message: "Internal server error",
			Status:  http.StatusInternalServerError,
		})
	}
}

// mapDomainErrorToHTTPStatus maps domain errors to HTTP status codes
func mapDomainErrorToHTTPStatus(err *domain.DomainError) int {
	switch err.Code {
	case domain.CodeNotFound, domain.CodeQuizNotFound:
		return http.StatusNotFound
	case domain.CodeInvalidInput, domain.CodeInvalidAnswer, domain.CodeInvalidCategory,
		domain.CodeValidation, domain.CodeMissingField, domain.CodeInvalidFormat, domain.CodeOutOfRange:
		return http.StatusBadRequest
	case domain.CodeUnauthorized:
		return http.StatusUnauthorized
	case domain.CodeLLMServiceError:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
