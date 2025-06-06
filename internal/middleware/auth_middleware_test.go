package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService implements the AuthService interface for testing
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GetGoogleLoginURL(state string) string {
	args := m.Called(state)
	return args.String(0)
}

func (m *MockAuthService) HandleGoogleCallback(ctx context.Context, code string, receivedState string, expectedState string) (string, string, *dto.AuthenticatedUser, error) {
	args := m.Called(ctx, code, receivedState, expectedState)
	return args.String(0), args.String(1), args.Get(2).(*dto.AuthenticatedUser), args.Error(3)
}

func (m *MockAuthService) CreateJWT(ctx context.Context, user *domain.User, ttl time.Duration, tokenType string) (string, error) {
	args := m.Called(ctx, user, ttl, tokenType)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) DecryptToken(encryptedToken string) (string, error) {
	args := m.Called(encryptedToken)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) EncryptToken(token string) (string, error) {
	args := m.Called(token)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshTokenString string) (string, string, error) {
	args := m.Called(ctx, refreshTokenString)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) ValidateJWT(ctx context.Context, token string) (*dto.AuthClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.AuthClaims), args.Error(1)
}

func TestJWTAuthMiddleware_ValidToken(t *testing.T) {
	mockAuthService := new(MockAuthService)
	claims := &dto.AuthClaims{
		UserID:    "test-user-id",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	mockAuthService.On("ValidateJWT", mock.Anything, "valid-token").Return(claims, nil)

	app := fiber.New()
	app.Use(middleware.Protected(mockAuthService))
	app.Get("/test", func(c *fiber.Ctx) error {
		userID := c.Locals(middleware.UserIDKey)
		assert.Equal(t, "test-user-id", userID)
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	mockAuthService.AssertExpectations(t)
}

func TestJWTAuthMiddleware_InvalidToken(t *testing.T) {
	mockAuthService := new(MockAuthService)
	mockAuthService.On("ValidateJWT", mock.Anything, "invalid-token").Return(nil, errors.New("invalid token"))

	app := fiber.New()
	app.Use(middleware.Protected(mockAuthService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	mockAuthService.AssertExpectations(t)
}

func TestJWTAuthMiddleware_MissingToken(t *testing.T) {
	mockAuthService := new(MockAuthService)

	app := fiber.New()
	app.Use(middleware.Protected(mockAuthService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	mockAuthService.AssertNotCalled(t, "ValidateJWT")
}

func TestJWTAuthMiddleware_InvalidAuthorizationHeader(t *testing.T) {
	mockAuthService := new(MockAuthService)

	app := fiber.New()
	app.Use(middleware.Protected(mockAuthService))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	mockAuthService.AssertNotCalled(t, "ValidateJWT")
}
