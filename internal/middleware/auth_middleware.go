package middleware

import (
	"fmt"
	"quiz-byte/internal/logger"  // Uncomment if logging is added here
	"quiz-byte/internal/service" // For AuthService
	"strings"

	"go.uber.org/zap" // Uncomment if logging is added here

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
				Message: err.Error(), // Include the actual error message
				Status:  fiber.StatusUnauthorized,
			})
		}

		// Ensure it's an access token
		if claims.TokenType != "access" { // "access" is the constant used in auth_service
			return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{ // 403 Forbidden might be more appropriate than 401 for wrong token type
				Code:    "INVALID_TOKEN_TYPE",
				Message: fmt.Sprintf("Invalid token type: expected access, got %s", claims.TokenType),
				Status:  fiber.StatusForbidden,
			})
		}

		c.Locals(UserIDKey, claims.UserID)

		return c.Next()
	}
}

// OptionalAuth is a middleware function that optionally authenticates a user.
// If a valid access token is provided, it sets the userID in the context.
// Otherwise, it proceeds without setting the userID, allowing for anonymous access.
func OptionalAuth(authService service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get(AuthorizationHeader)

		// If no Authorization header, proceed as anonymous
		if authHeader == "" {
			return c.Next()
		}

		// Check if the Authorization header uses the Bearer schema
		if !strings.HasPrefix(authHeader, BearerSchema) {
			logger.Get().Debug("OptionalAuth: Authorization scheme is not Bearer, proceeding as anonymous.", zap.String("header", authHeader))
			return c.Next()
		}

		tokenString := strings.TrimPrefix(authHeader, BearerSchema)
		if tokenString == "" {
			logger.Get().Debug("OptionalAuth: Token is empty after trimming Bearer prefix, proceeding as anonymous.")
			return c.Next()
		}

		claims, err := authService.ValidateJWT(c.Context(), tokenString)
		if err != nil {
			logger.Get().Debug("OptionalAuth: JWT validation failed, proceeding as anonymous.", zap.Error(err), zap.String("token", tokenString))
			return c.Next()
		}

		// Ensure it's an access token
		if claims.TokenType != "access" { // "access" is the constant used in auth_service
			logger.Get().Debug("OptionalAuth: Invalid token type, expected access token, proceeding as anonymous.", zap.String("tokenType", claims.TokenType))
			return c.Next()
		}

		// If all checks pass, set UserID in locals
		c.Locals(UserIDKey, claims.UserID)
		logger.Get().Debug("OptionalAuth: User authenticated.", zap.String("userID", claims.UserID))

		return c.Next()
	}
}
