package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"    // For AuthClaims and AuthenticatedUser
	"quiz-byte/internal/logger" // Added
	"quiz-byte/internal/util" // For ULID generation
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap" // Added
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	tokenTypeAccess   = "access"
	tokenTypeRefresh  = "refresh"
)

var (
	ErrInvalidAuthState      = errors.New("invalid oauth state")
	ErrFailedToExchangeToken = errors.New("failed to exchange oauth token")
	ErrFailedToGetUserInfo   = errors.New("failed to get user info from google")
	ErrInvalidJWTToken       = errors.New("invalid jwt token")
	ErrEncryptionFailed      = errors.New("failed to encrypt token")
	ErrDecryptionFailed      = errors.New("failed to decrypt token")
)

// AuthService defines the interface for authentication operations.
type AuthService interface {
	GetGoogleLoginURL(state string) string
	HandleGoogleCallback(ctx context.Context, code string, receivedState string, expectedState string) (accessToken string, refreshToken string, user *dto.AuthenticatedUser, err error) // Changed return type
	ValidateJWT(ctx context.Context, tokenString string) (*dto.AuthClaims, error)
	CreateJWT(ctx context.Context, user *domain.User, ttl time.Duration, tokenType string) (string, error)
	RefreshToken(ctx context.Context, refreshTokenString string) (newAccessToken string, newRefreshToken string, err error)
	EncryptToken(token string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
}

type authServiceImpl struct {
	userRepo      domain.UserRepository // Changed to domain.UserRepository
	oauth2Config  *oauth2.Config
	authCfg       config.AuthConfig // Changed from appConfig
	encryptionKey []byte
}

// NewAuthService creates a new instance of AuthService.
func NewAuthService(userRepo domain.UserRepository, authCfg config.AuthConfig) (AuthService, error) { // Changed param type
	if len(authCfg.JWT.SecretKey) == 0 {
		return nil, errors.New("encryption key for auth service is not configured (use JWT.SecretKey or a dedicated one)")
	}
	var encKey []byte
	if len(authCfg.JWT.SecretKey) >= 32 {
		encKey = []byte(authCfg.JWT.SecretKey[:32])
	} else {
		return nil, errors.New("encryption key must be at least 32 bytes long")
	}

	return &authServiceImpl{
		userRepo: userRepo,
		oauth2Config: &oauth2.Config{
			ClientID:     authCfg.GoogleOAuth.ClientID,
			ClientSecret: authCfg.GoogleOAuth.ClientSecret,
			RedirectURL:  authCfg.GoogleOAuth.RedirectURL,
			Scopes:       []string{"https_://www.googleapis.com/auth/userinfo.email", "https_://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     google.Endpoint,
		},
		authCfg:       authCfg,
		encryptionKey: encKey,
	}, nil
}

func (s *authServiceImpl) GetGoogleLoginURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (s *authServiceImpl) HandleGoogleCallback(ctx context.Context, code string, receivedState string, expectedState string) (string, string, *dto.AuthenticatedUser, error) { // Changed return type
	appLogger := logger.Get()
	if receivedState != expectedState {
		return "", "", nil, ErrInvalidAuthState
	}

	googleToken, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", ErrFailedToExchangeToken, err)
	}

	client := s.oauth2Config.Client(ctx, googleToken)
	resp, err := client.Get(googleUserInfoURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", ErrFailedToGetUserInfo, err)
	}
	defer resp.Body.Close()

	var userInfo dto.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	if userInfo.ID == "" || userInfo.Email == "" {
		return "", "", nil, errors.New("google user info is incomplete")
	}

	domainUser, err := s.userRepo.GetUserByGoogleID(ctx, userInfo.ID)
	// Check for actual error vs not found (nil, nil from repo)
	if err != nil && domainUser == nil { // Assuming GetUserByGoogleID returns (nil, actualError) for DB errors
		return "", "", nil, fmt.Errorf("error fetching user by google_id: %w", err)
	}
	// If domainUser is nil and err is nil, it means user not found, which is handled next.

	// Token encryption logic remains, but these encrypted tokens are not directly saved
	// by the domain repository's CreateUser/UpdateUser methods.
	// Their persistence is deferred / handled by a future specialized repository method.
	_, err = s.EncryptToken(googleToken.AccessToken) // Keep encryption to see if it works, but don't use the result for domainUser
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to encrypt access token: %w", err)
	}
	if googleToken.RefreshToken != "" {
		_, err = s.EncryptToken(googleToken.RefreshToken)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	now := time.Now()
	if domainUser == nil { // User not found, create new domain user
		domainUser = &domain.User{
			ID:                util.NewULID(),
			GoogleID:          userInfo.ID,
			Email:             userInfo.Email,
			Name:              userInfo.Name,
			ProfilePictureURL: userInfo.Picture,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := s.userRepo.CreateUser(ctx, domainUser); err != nil {
			return "", "", nil, fmt.Errorf("failed to create user: %w", err)
		}
		appLogger.Info("New user created via Google OAuth", zap.String("userID", domainUser.ID), zap.String("email", domainUser.Email))
	} else { // User found, update profile info if changed
		domainUser.Email = userInfo.Email
		domainUser.Name = userInfo.Name
		domainUser.ProfilePictureURL = userInfo.Picture
		domainUser.UpdatedAt = now
		if err := s.userRepo.UpdateUser(ctx, domainUser); err != nil {
			return "", "", nil, fmt.Errorf("failed to update user: %w", err)
		}
		appLogger.Info("User logged in via Google OAuth", zap.String("userID", domainUser.ID), zap.String("email", domainUser.Email))
	}

	accessToken, err := s.CreateJWT(ctx, domainUser, s.authCfg.JWT.AccessTokenTTL, tokenTypeAccess)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create access token: %w", err)
	}
	refreshToken, err := s.CreateJWT(ctx, domainUser, s.authCfg.JWT.RefreshTokenTTL, tokenTypeRefresh)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	// Map domainUser to dto.AuthenticatedUser for the return type.
	authenticatedUserData := &dto.AuthenticatedUser{
		ID:                domainUser.ID,
		Email:             domainUser.Email,
		Name:              domainUser.Name,
		ProfilePictureURL: domainUser.ProfilePictureURL,
	}

	return accessToken, refreshToken, authenticatedUserData, nil
}

func (s *authServiceImpl) CreateJWT(ctx context.Context, user *domain.User, ttl time.Duration, tokenType string) (string, error) {
	claims := dto.AuthClaims{
		UserID:    user.ID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.authCfg.JWT.SecretKey))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *authServiceImpl) ValidateJWT(ctx context.Context, tokenString string) (*dto.AuthClaims, error) {
	appLogger := logger.Get()
	token, err := jwt.ParseWithClaims(tokenString, &dto.AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.authCfg.JWT.SecretKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			appLogger.Warn("JWT token expired",
				zap.Error(err),
				zap.String("token_snippet", tokenString[:min(len(tokenString), 20)]+"..."))
		} else {
			appLogger.Warn("JWT validation failed",
				zap.Error(err),
				zap.String("token_snippet", tokenString[:min(len(tokenString), 20)]+"..."))
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidJWTToken, err)
	}

	if claims, ok := token.Claims.(*dto.AuthClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidJWTToken
}

func (s *authServiceImpl) RefreshToken(ctx context.Context, refreshTokenString string) (string, string, error) {
	appLogger := logger.Get()
	claims, err := s.ValidateJWT(ctx, refreshTokenString)
	if err != nil {
		appLogger.Warn("Refresh token validation failed during initial check",
			zap.Error(err),
			zap.String("refresh_token_snippet", refreshTokenString[:min(len(refreshTokenString), 20)]+"..."))
		return "", "", fmt.Errorf("invalid refresh token: %w", err)
	}
	if claims.TokenType != tokenTypeRefresh {
		return "", "", errors.New("not a refresh token")
	}

	domainUser, err := s.userRepo.GetUserByID(ctx, claims.UserID) // Returns domain.User
	if err != nil || domainUser == nil {
		// Handle domain.NewNotFoundError if userRepo returns that for not found
		if errors.Is(err, &domain.NotFoundError{}) || domainUser == nil && err == nil { // Check custom error type if applicable
			appLogger.Warn("User not found for refresh token", zap.String("userID", claims.UserID))
			return "", "", domain.NewNotFoundError(fmt.Sprintf("user %s not found for refresh token", claims.UserID))
		}
		appLogger.Error("Error fetching user for refresh token", zap.String("userID", claims.UserID), zap.Error(err))
		return "", "", fmt.Errorf("error fetching user for refresh token: %w", err)
	}

	newAccessToken, err := s.CreateJWT(ctx, domainUser, s.authCfg.JWT.AccessTokenTTL, tokenTypeAccess)
	if err != nil {
		return "", "", fmt.Errorf("failed to create new access token: %w", err)
	}
	newRefreshToken, err := s.CreateJWT(ctx, domainUser, s.authCfg.JWT.RefreshTokenTTL, tokenTypeRefresh)
	if err != nil {
		return "", "", fmt.Errorf("failed to create new refresh token: %w", err)
	}

	appLogger.Info("JWT token refreshed", zap.String("userID", domainUser.ID))
	return newAccessToken, newRefreshToken, nil
}

// EncryptToken encrypts a token using AES-GCM.
func (s *authServiceImpl) EncryptToken(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken decrypts a token using AES-GCM.
func (s *authServiceImpl) DecryptToken(encryptedToken string) (string, error) {
	if encryptedToken == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("%w: ciphertext too short", ErrDecryptionFailed)
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	return string(plaintext), nil
}
