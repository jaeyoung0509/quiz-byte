package middleware_test

import (
	"context"
	"errors"
	"net/http/httptest"
	// Adjust the import path if auth.Claims is located elsewhere, e.g., internal/auth/claims
	"quiz-byte/internal/domain/auth"
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/service" // For the AuthService interface
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5" // For jwt.RegisteredClaims
	"github.com/stretchr/testify/assert"
	// "go.uber.org/mock/gomock" // Not using generated mocks directly in this step
)

// Manual MockAuthService for testing middleware.AuthService interface
type ManualMockAuthService struct {
	ValidateJWTFunc func(ctx context.Context, tokenString string) (*auth.Claims, error)
}

func (m *ManualMockAuthService) GenerateJWT(userID, tokenType string) (string, error) {
	panic("not implemented in mock")
}

func (m *ManualMockAuthService) ValidateJWT(ctx context.Context, tokenString string) (*auth.Claims, error) {
	if m.ValidateJWTFunc != nil {
		return m.ValidateJWTFunc(ctx, tokenString)
	}
	return nil, errors.New("ValidateJWTFunc not set on mock")
}

func (m *ManualMockAuthService) GenerateAndStoreTokens(ctx context.Context, userID string, prevRefreshTokenID string) (accessToken string, refreshToken string, err error) {
	panic("not implemented in mock")
}

func (m *ManualMockAuthService) ValidateAndRefreshTokens(ctx context.Context, oldAccessTokenString string, oldRefreshTokenString string) (newAccessToken string, newRefreshToken string, err error) {
	panic("not implemented in mock")
}

func (m *ManualMockAuthService) RevokeToken(ctx context.Context, tokenID string, tokenType string) error {
	panic("not implemented in mock")
}

func (m *ManualMockAuthService) GetUserIDFromToken(ctx context.Context, tokenString string) (string, error) {
	panic("not implemented in mock")
}


func TestOptionalAuth(t *testing.T) {

	mockAuthSvc := &ManualMockAuthService{}

	tests := []struct {
		name                string
		authHeader          string
		setupMock           func(mockSvc *ManualMockAuthService)
		expectedStatus      int
		expectedUserIDLocal interface{}
		expectNextCalled    bool
	}{
		{
			name:           "No Auth Header",
			authHeader:     "",
			setupMock:      func(mockSvc *ManualMockAuthService) {},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: nil,
			expectNextCalled: true,
		},
		{
			name:       "Valid Access Token",
			authHeader: "Bearer valid_access_token",
			setupMock: func(mockSvc *ManualMockAuthService) {
				mockSvc.ValidateJWTFunc = func(ctx context.Context, tokenString string) (*auth.Claims, error) {
					assert.Equal(t, "valid_access_token", tokenString)
					return &auth.Claims{UserID: "user123", TokenType: "access", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour))}}, nil
				}
			},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: "user123",
			expectNextCalled: true,
		},
		{
			name:       "Invalid Token (validation error)",
			authHeader: "Bearer invalid_token",
			setupMock: func(mockSvc *ManualMockAuthService) {
				mockSvc.ValidateJWTFunc = func(ctx context.Context, tokenString string) (*auth.Claims, error) {
					assert.Equal(t, "invalid_token", tokenString)
					return nil, errors.New("invalid token")
				}
			},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: nil,
			expectNextCalled: true,
		},
		{
			name:       "Valid Refresh Token instead of Access",
			authHeader: "Bearer valid_refresh_token",
			setupMock: func(mockSvc *ManualMockAuthService) {
				mockSvc.ValidateJWTFunc = func(ctx context.Context, tokenString string) (*auth.Claims, error) {
					assert.Equal(t, "valid_refresh_token", tokenString)
					// TokenType is "refresh"
					return &auth.Claims{UserID: "user456", TokenType: "refresh", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour))}}, nil
				}
			},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: nil,
			expectNextCalled: true,
		},
		{
			name:           "Malformed Auth Header - No Bearer",
			authHeader:     "Basic some_token",
			setupMock:      func(mockSvc *ManualMockAuthService) {},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: nil,
			expectNextCalled: true,
		},
		{
			name:       "Malformed Auth Header - Bearer No Token",
			authHeader: "Bearer ",
			setupMock:      func(mockSvc *ManualMockAuthService) {},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: nil,
			expectNextCalled: true,
		},
		{
			name:       "Empty Token String",
			authHeader: "Bearer ", // Covered by "Malformed Auth Header - Bearer No Token" effectively
			setupMock:      func(mockSvc *ManualMockAuthService) {},
			expectedStatus: fiber.StatusOK,
			expectedUserIDLocal: nil,
			expectNextCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New() // Create new app for each test to reset state
			tc.setupMock(mockAuthSvc)

			nextHandlerCalled := false
			var userIDLocalValue interface{}

			// Setup route with middleware and a handler to capture locals and check if called
			app.Get("/test_optional_auth", middleware.OptionalAuth(mockAuthSvc), func(c *fiber.Ctx) error {
				nextHandlerCalled = true
				userIDLocalValue = c.Locals(middleware.UserIDKey)
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test_optional_auth", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			resp, err := app.Test(req, -1)

			assert.NoError(t, err, "app.Test should not return an error")
			if err == nil { // Only assert status if no app.Test error
				assert.Equal(t, tc.expectedStatus, resp.StatusCode, "HTTP status code mismatch")
			}

			assert.True(t, nextHandlerCalled, "Next handler was not called")
			assert.Equal(t, tc.expectedUserIDLocal, userIDLocalValue, "UserID in Ctx.Locals mismatch")
		})
	}
}
