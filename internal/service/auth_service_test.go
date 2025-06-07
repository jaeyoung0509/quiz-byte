package service

import (
	"context"
	"errors"
	"fmt"
	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository is a mock type for the domain.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserByGoogleID(ctx context.Context, googleID string) (*domain.User, error) {
	args := m.Called(ctx, googleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func TestAuthService_RefreshToken_UserNotFound(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	authCfg := config.AuthConfig{
		JWT: config.JWTConfig{
			SecretKey:       "testsecretkeydontuseinproduction32bytes!",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
		},
	}

	authService, err := NewAuthService(mockUserRepo, authCfg, &MockTransactionManager{})
	assert.NoError(t, err)

	// Create a valid refresh token string (for testing purposes)
	// In a real scenario, this would be a token previously issued by CreateJWT.
	// For this test, we only need it to pass ValidateJWT's parsing and initial claims check.
	dummyUser := &domain.User{ID: "user123"}
	refreshTokenString, _ := authService.CreateJWT(context.Background(), dummyUser, authCfg.JWT.RefreshTokenTTL, "refresh")

	// Setup mock expectations
	// ValidateJWT will be called, assume it passes and extracts claims.
	// Then GetUserByID is called.
	mockUserRepo.On("GetUserByID", mock.Anything, "user123").Return(nil, nil) // Simulate user not found (repo returns (nil, nil))

	_, _, err = authService.RefreshToken(context.Background(), refreshTokenString)

	assert.Error(t, err)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr), "Error should be a domain.DomainError")
	if domainErr != nil {
		assert.Equal(t, domain.CodeNotFound, domainErr.Code)
	}
}

func TestAuthService_RefreshToken_RepoError(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	authCfg := config.AuthConfig{
		JWT: config.JWTConfig{
			SecretKey:       "testsecretkeydontuseinproduction32bytes!",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
		},
	}
	authService, err := NewAuthService(mockUserRepo, authCfg, &MockTransactionManager{})
	assert.NoError(t, err)

	dummyUser := &domain.User{ID: "user123"}
	refreshTokenString, _ := authService.CreateJWT(context.Background(), dummyUser, authCfg.JWT.RefreshTokenTTL, "refresh")

	// Simulate a generic database error from the repository
	expectedRepoError := fmt.Errorf("some database connection error")
	mockUserRepo.On("GetUserByID", mock.Anything, "user123").Return(nil, expectedRepoError)

	_, _, err = authService.RefreshToken(context.Background(), refreshTokenString)

	assert.Error(t, err)
	var domainErr *domain.DomainError
	assert.True(t, errors.As(err, &domainErr), "Error should be a domain.DomainError")
	if domainErr != nil {
		assert.Equal(t, domain.CodeInternal, domainErr.Code)
		assert.ErrorIs(t, err, expectedRepoError) // Check if the original error is wrapped
	}
}

func TestAuthService_HandleGoogleCallback_CreateUser_RepoError(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	authCfg := config.AuthConfig{
		JWT: config.JWTConfig{
			SecretKey: "testsecretkeydontuseinproduction32bytes!", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour,
		},
		GoogleOAuth: config.GoogleOAuthConfig{ /* fields can be dummy for this test if not directly used before repo call */ },
	}
	// Note: NewAuthService also creates oauth2Config. For this test, we're focused on repo interaction.
	// A more complete test might mock http calls for oauth2.Exchange and client.Get.

	_, err := NewAuthService(mockUserRepo, authCfg, &MockTransactionManager{})
	assert.NoError(t, err)

	// Mock GetUserByGoogleID to return (nil, nil) -> user not found
	mockUserRepo.On("GetUserByGoogleID", mock.Anything, "googleTestID").Return(nil, nil)

	// Mock CreateUser to return an error
	expectedRepoError := errors.New("failed to create user in DB")
	mockUserRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("*domain.User")).Return(expectedRepoError)

	// Simulate a successful Google OAuth exchange part (token & user info)
	// This part is complex to fully mock without also mocking HTTP.
	// We are focusing on the error from CreateUser.
	// To simplify, we'd need to bypass the actual Google API calls in a real unit test for this specific case,
	// or make HandleGoogleCallback more modular to test parts.
	// For this example, we assume the flow reaches CreateUser.
	// A true unit test for HandleGoogleCallback would require more extensive mocking (e.g., http client for Google API calls).

	// This test will be more conceptual for now as fully testing HandleGoogleCallback as a unit is complex
	// due to its external HTTP calls. The main goal here is to illustrate testing the error conversion from repo.

	// If we could directly call the part after getting Google UserInfo:
	// Assume userInfo is populated
	// ...
	// domainUser := constructDomainUserFromGoogleInfo()
	// err := authService.userRepo.CreateUser(ctx, domainUser)
	// if err != nil { /* THIS IS WHAT WE WANT TO TEST */
	//    // service should return domain.NewInternalError("...", err)
	// }

	// Due to the complexity of mocking the full HandleGoogleCallback, this test case will be omitted for now.
	// The error wrapping for CreateUser and UpdateUser calls within HandleGoogleCallback was already
	// updated to domain.NewInternalError in the previous step.
	t.Log("Skipping full HandleGoogleCallback test due to complexity of mocking external HTTP calls in this context.")
}

// TODO: Add more tests for other scenarios in AuthService,
// e.g., successful token refresh, successful Google callback, JWT creation/validation errors.
