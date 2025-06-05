package middleware

import (
	"quiz-byte/internal/domain"
	"quiz-byte/internal/validation"

	"github.com/gofiber/fiber/v2"
)

// ValidationMiddleware provides request validation middleware
type ValidationMiddleware struct {
	validator *validation.Validator
}

// NewValidationMiddleware creates a new validation middleware instance
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: validation.NewValidator(),
	}
}

// ValidateSubCategory validates sub_category parameter from query or path
func (vm *ValidationMiddleware) ValidateSubCategory() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to get from path parameter first, then query parameter
		subCategory := c.Params("subCategory")
		if subCategory == "" {
			subCategory = c.Query("sub_category")
		}

		if errors := vm.validator.ValidateSubCategory(subCategory); len(errors) > 0 {
			return errors // This will be handled by ErrorHandler middleware
		}

		// Store validated value in context for handlers to use
		c.Locals("validated_sub_category", subCategory)
		return c.Next()
	}
}

// ValidateBulkQuizzesParams validates bulk quizzes request parameters
func (vm *ValidationMiddleware) ValidateBulkQuizzesParams() fiber.Handler {
	return func(c *fiber.Ctx) error {
		subCategory := c.Query("sub_category")

		// Parse count with default value
		count := 10 // default
		if countStr := c.Query("count"); countStr != "" {
			if parsedCount, err := parseCount(countStr); err != nil {
				return domain.ValidationErrors{
					domain.NewInvalidFormatError("count", countStr),
				}
			} else {
				count = parsedCount
			}
		}

		if errors := vm.validator.ValidateBulkQuizzesRequest(subCategory, count); len(errors) > 0 {
			return errors
		}

		// Store validated values in context
		c.Locals("validated_sub_category", subCategory)
		c.Locals("validated_count", count)
		return c.Next()
	}
}

// parseCount parses count parameter with validation
func parseCount(countStr string) (int, error) {
	count := 0
	for _, char := range countStr {
		if char < '0' || char > '9' {
			return 0, domain.NewValidationError("count must be a number")
		}
		count = count*10 + int(char-'0')
		if count > 50 { // Early termination for efficiency
			return 0, domain.NewValidationError("count exceeds maximum value")
		}
	}
	if count == 0 {
		return 0, domain.NewValidationError("count must be greater than 0")
	}
	return count, nil
}
