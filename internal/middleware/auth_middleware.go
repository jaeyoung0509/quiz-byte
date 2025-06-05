package middleware

import (
	"quiz-byte/internal/service" // For AuthService
	"strings"
	// "quiz-byte/internal/logger" // Uncomment if logging is added here
	// "go.uber.org/zap" // Uncomment if logging is added here

	"github.com/gofiber/fiber/v2"
)

const (
	AuthorizationHeader = "Authorization"
	BearerSchema        = "Bearer "
	UserIDKey           = "userID" // Key for storing UserID in fiber.Ctx locals
)

// Protected is a middleware function that protects routes by requiring a valid JWT.
// It validates the token using the provided AuthService and sets the userID in the context.
func Protected(authService service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get(AuthorizationHeader)
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{ // Using existing middleware.ErrorResponse
				Code:    "MISSING_AUTH_HEADER",
				Message: "Authorization header is missing",
				Status:  fiber.StatusUnauthorized,
			})
		}

		if !strings.HasPrefix(authHeader, BearerSchema) {
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
				Code:    "INVALID_AUTH_SCHEME",
				Message: "Authorization scheme is not Bearer",
				Status:  fiber.StatusUnauthorized,
			})
		}

		tokenString := strings.TrimPrefix(authHeader, BearerSchema)
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
				Code:    "EMPTY_TOKEN",
				Message: "Token is empty",
				Status:  fiber.StatusUnauthorized,
			})
		}

		claims, err := authService.ValidateJWT(c.Context(), tokenString)
		if err != nil {
			// For debugging, could log the actual error:
			// logger.Get().Debug("JWT Validation Error", zap.Error(err), zap.String("token", tokenString))
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
				Code:    "INVALID_TOKEN",
				Message: "Token is invalid or expired",
				Status:  fiber.StatusUnauthorized,
			})
		}

		// Ensure it's an access token
		if claims.TokenType != "access" { // "access" is the constant used in auth_service
			return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{ // 403 Forbidden might be more appropriate than 401 for wrong token type
				Code:    "INVALID_TOKEN_TYPE",
				Message: "Invalid token type provided; expected access token",
				Status:  fiber.StatusForbidden,
			})
		}

		c.Locals(UserIDKey, claims.UserID)

		return c.Next()
	}
}
