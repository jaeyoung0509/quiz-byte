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
	"quiz-byte/internal/domain" // For custom errors like UserNotFoundError
	"quiz-byte/internal/dto"    // For AuthClaims
	"quiz-byte/internal/logger" // Added
	"quiz-byte/internal/repository"
	"quiz-byte/internal/repository/models"
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
	HandleGoogleCallback(ctx context.Context, code string, receivedState string, expectedState string) (accessToken string, refreshToken string, user *models.User, err error)
	ValidateJWT(ctx context.Context, tokenString string) (*dto.AuthClaims, error)
	CreateJWT(ctx context.Context, user *models.User, ttl time.Duration, tokenType string) (string, error)
	RefreshToken(ctx context.Context, refreshTokenString string) (newAccessToken string, newRefreshToken string, err error)
	EncryptToken(token string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
	// Logout might be mostly client-side, but a hook can be here if server-side invalidation (e.g. blacklist) is added
}

type authServiceImpl struct {
	userRepo      repository.UserRepository
	oauth2Config  *oauth2.Config
	appConfig     *config.Config
	encryptionKey []byte // Should be 32 bytes for AES-256
}

// NewAuthService creates a new instance of AuthService.
func NewAuthService(userRepo repository.UserRepository, appConfig *config.Config) (AuthService, error) {
	if len(appConfig.JWT.SecretKey) == 0 { // Also use a dedicated key for encryption
		return nil, errors.New("encryption key for auth service is not configured (use JWT.SecretKey or a dedicated one)")
	}
	// For simplicity, using first 32 bytes of JWT secret if available, or a dedicated key.
	// Best practice: use a dedicated, randomly generated key for encryption stored in config.
	var encKey []byte
	if len(appConfig.JWT.SecretKey) >= 32 {
		encKey = []byte(appConfig.JWT.SecretKey[:32])
	} else {
		// This is not secure for production. A proper key should be configured.
		// For now, let's pad or error. Erroring is safer.
		return nil, errors.New("encryption key must be at least 32 bytes long")
	}

	return &authServiceImpl{
		userRepo: userRepo,
		oauth2Config: &oauth2.Config{
			ClientID:     appConfig.GoogleOAuth.ClientID,
			ClientSecret: appConfig.GoogleOAuth.ClientSecret,
			RedirectURL:  appConfig.GoogleOAuth.RedirectURL,
			Scopes:       []string{"https_://www.googleapis.com/auth/userinfo.email", "https_://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     google.Endpoint,
		},
		appConfig:     appConfig,
		encryptionKey: encKey,
	}, nil
}

func (s *authServiceImpl) GetGoogleLoginURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce) // Request refresh token
}

func (s *authServiceImpl) HandleGoogleCallback(ctx context.Context, code string, receivedState string, expectedState string) (string, string, *models.User, error) {
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

	user, err := s.userRepo.GetUserByGoogleID(ctx, userInfo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) && user == nil { // Check for actual error vs not found
		return "", "", nil, fmt.Errorf("error fetching user by google_id: %w", err)
	}

	encryptedAccessToken, err := s.EncryptToken(googleToken.AccessToken)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to encrypt access token: %w", err)
	}
	var encryptedRefreshToken string
	if googleToken.RefreshToken != "" {
		encryptedRefreshToken, err = s.EncryptToken(googleToken.RefreshToken)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	now := time.Now()
	if user == nil { // User not found, create new user
		newUser := &models.User{
			ID:                    util.NewULID(),
			GoogleID:              userInfo.ID,
			Email:                 userInfo.Email,
			Name:                  util.StringToNullString(userInfo.Name),
			ProfilePictureURL:     util.StringToNullString(userInfo.Picture),
			EncryptedAccessToken:  util.StringToNullString(encryptedAccessToken),
			EncryptedRefreshToken: util.StringToNullString(encryptedRefreshToken),
			TokenExpiresAt:        util.TimeToNullTime(googleToken.Expiry),
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		if err := s.userRepo.CreateUser(ctx, newUser); err != nil {
			return "", "", nil, fmt.Errorf("failed to create user: %w", err)
		}
		user = newUser
		appLogger.Info("New user created via Google OAuth", zap.String("userID", user.ID), zap.String("email", user.Email))
	} else { // User found, update tokens and profile info if changed
		user.Email = userInfo.Email // Google is source of truth for email
		user.Name = util.StringToNullString(userInfo.Name)
		user.ProfilePictureURL = util.StringToNullString(userInfo.Picture)
		user.EncryptedAccessToken = util.StringToNullString(encryptedAccessToken)
		if encryptedRefreshToken != "" { // Only update refresh token if a new one was provided
			user.EncryptedRefreshToken = util.StringToNullString(encryptedRefreshToken)
		}
		user.TokenExpiresAt = util.TimeToNullTime(googleToken.Expiry)
		user.UpdatedAt = now
		if err := s.userRepo.UpdateUser(ctx, user); err != nil {
			return "", "", nil, fmt.Errorf("failed to update user: %w", err)
		}
		appLogger.Info("User logged in via Google OAuth", zap.String("userID", user.ID), zap.String("email", user.Email))
	}

	accessToken, err := s.CreateJWT(ctx, user, s.appConfig.JWT.AccessTokenTTL, tokenTypeAccess)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create access token: %w", err)
	}
	refreshToken, err := s.CreateJWT(ctx, user, s.appConfig.JWT.RefreshTokenTTL, tokenTypeRefresh)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return accessToken, refreshToken, user, nil
}

func (s *authServiceImpl) CreateJWT(ctx context.Context, user *models.User, ttl time.Duration, tokenType string) (string, error) {
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
	return token.SignedString([]byte(s.appConfig.JWT.SecretKey))
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
		return []byte(s.appConfig.JWT.SecretKey), nil
	})

	if err != nil {
		// Log specific errors like token expiry
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

	user, err := s.userRepo.GetUserByID(ctx, claims.UserID)
	if err != nil || user == nil {
		appLogger.Error("User not found for refresh token", zap.String("userID", claims.UserID), zap.Error(err))
		return "", "", domain.NewNotFoundError(fmt.Sprintf("User %s not found for refresh token", claims.UserID))
	}

	newAccessToken, err := s.CreateJWT(ctx, user, s.appConfig.JWT.AccessTokenTTL, tokenTypeAccess)
	if err != nil {
		return "", "", fmt.Errorf("failed to create new access token: %w", err)
	}
	newRefreshToken, err := s.CreateJWT(ctx, user, s.appConfig.JWT.RefreshTokenTTL, tokenTypeRefresh)
	if err != nil {
		return "", "", fmt.Errorf("failed to create new refresh token: %w", err)
	}

	appLogger.Info("JWT token refreshed", zap.String("userID", user.ID))
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
