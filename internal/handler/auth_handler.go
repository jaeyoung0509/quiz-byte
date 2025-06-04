package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	// "net/http" // Not directly used, can be removed
	"quiz-byte/internal/config"
	"quiz-byte/internal/middleware" // For middleware.ErrorResponse
	"quiz-byte/internal/service"
	"quiz-byte/internal/logger" // Added
	"time"
	"go.uber.org/zap" // Added

	"github.com/gofiber/fiber/v2"
)

const (
	oauthStateCookieName = "oauthstate"
	// Note: Frontend URL should come from config for redirects
)

type AuthHandler struct {
	authService service.AuthService
	appConfig   *config.Config // For frontend URL, cookie settings
}

func NewAuthHandler(authService service.AuthService, appConfig *config.Config) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		appConfig:   appConfig,
	}
}

// GoogleLogin initiates the Google OAuth2 login flow.
// @Summary Initiate Google Login
// @Description Redirects the user to Google's OAuth2 consent page.
// @Tags auth
// @Success 302 {string} string "Redirects to Google"
// @Router /auth/google/login [get]
func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error {
	appLogger := logger.Get()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		appLogger.Error("Failed to generate random state for OAuth", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "OAUTH_STATE_GENERATION_ERROR", Message: "Could not generate state for OAuth flow", Status: fiber.StatusInternalServerError,
		})
	}
	state := base64.URLEncoding.EncodeToString(b)
	appLogger.Info("Google login process initiated", zap.String("state", state))

	c.Cookie(&fiber.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		Secure:   c.Secure(),
		SameSite: "Lax",
		Path:     "/",
	})

	loginURL := h.authService.GetGoogleLoginURL(state)
	return c.Redirect(loginURL, fiber.StatusTemporaryRedirect)
}

// GoogleCallback handles the callback from Google OAuth2.
// @Summary Google OAuth2 Callback
// @Description Handles user authentication after Google login, issues JWTs.
// @Tags auth
// @Param code query string true "Authorization code from Google"
// @Param state query string true "State string for CSRF protection"
// @Success 200 {object} map[string]string "Contains access_token and refresh_token"
// @Failure 400 {object} middleware.ErrorResponse "Invalid state or code"
// @Failure 500 {object} middleware.ErrorResponse "Internal server error"
// @Router /auth/google/callback [get]
func (h *AuthHandler) GoogleCallback(c *fiber.Ctx) error {
	appLogger := logger.Get()
	code := c.Query("code")
	receivedState := c.Query("state")
	expectedState := c.Cookies(oauthStateCookieName)

	c.Cookie(&fiber.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		Secure:   c.Secure(),
		SameSite: "Lax",
		Path:     "/",
	})

	if code == "" {
		appLogger.Warn("Authorization code missing in Google OAuth callback")
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "MISSING_CODE", Message: "Authorization code is missing", Status: fiber.StatusBadRequest,
		})
	}
	if receivedState == "" || expectedState == "" || receivedState != expectedState {
		appLogger.Warn("OAuth state mismatch", zap.String("received", receivedState), zap.String("expected", expectedState))
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "INVALID_STATE", Message: "OAuth state mismatch or missing", Status: fiber.StatusBadRequest,
		})
	}

	accessToken, refreshToken, user, err := h.authService.HandleGoogleCallback(c.Context(), code, receivedState, expectedState)
	if err != nil {
		appLogger.Error("Failed to handle Google callback in authService",
			zap.Error(err),
			zap.String("code", code),
			zap.String("received_state", receivedState))
		if errors.Is(err, service.ErrInvalidAuthState) || errors.Is(err, service.ErrFailedToExchangeToken) {
			return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
				Code: "OAUTH_CALLBACK_ERROR", Message: err.Error(), Status: fiber.StatusBadRequest,
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(middleware.ErrorResponse{
			Code: "OAUTH_PROCESSING_ERROR", Message: "Error processing Google login", Status: fiber.StatusInternalServerError,
		})
	}

	if user != nil {
		appLogger.Info("Google OAuth callback successful, tokens issued", zap.String("userID", user.ID))
	} else {
		// This case should ideally not happen if HandleGoogleCallback returns a user on success
		appLogger.Error("User object is nil after successful Google OAuth callback")
	}

	// Here, instead of returning tokens in JSON, you might set them in HttpOnly cookies
	// and redirect to the frontend application.
	// Example:
	// frontendURL := h.appConfig.Server.FrontendURL // Assuming you have this in your config
	// if frontendURL == "" { frontendURL = "/" } // Default redirect
	//
	// c.Cookie(&fiber.Cookie{Name: "access_token", Value: accessToken, Path: "/", HttpOnly: true, Secure: c.Secure(), SameSite: "Lax", Expires: time.Now().Add(h.authService.GetAccessTokenTTL())})
	// c.Cookie(&fiber.Cookie{Name: "refresh_token", Value: refreshToken, Path: "/", HttpOnly: true, Secure: c.Secure(), SameSite: "Lax", Expires: time.Now().Add(h.authService.GetRefreshTokenTTL())})
	// return c.Redirect(frontendURL, fiber.StatusTemporaryRedirect)
	//
	// For now, returning as JSON as per original plan for API-style interaction.
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// RefreshToken generates new access and refresh tokens using a valid refresh token.
// @Summary Refresh JWT tokens
// @Description Provides a new access token and potentially a new refresh token if the provided refresh token is valid.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body map[string]string true "JSON object with 'refresh_token'"
// @Success 200 {object} map[string]string "Contains 'access_token' and 'refresh_token'"
// @Failure 400 {object} middleware.ErrorResponse "Refresh token missing or invalid format"
// @Failure 401 {object} middleware.ErrorResponse "Refresh token invalid or expired"
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	appLogger := logger.Get()
	var reqBody map[string]string
	if err := c.BodyParser(&reqBody); err != nil {
		appLogger.Warn("Failed to parse request body for token refresh", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "INVALID_REQUEST_BODY", Message: "Invalid request body", Status: fiber.StatusBadRequest,
		})
	}

	refreshTokenString, ok := reqBody["refresh_token"]
	if !ok || refreshTokenString == "" {
		appLogger.Warn("Refresh token missing in request body")
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "MISSING_REFRESH_TOKEN", Message: "Refresh token is missing in request body", Status: fiber.StatusBadRequest,
		})
	}

	newAccessToken, newRefreshToken, err := h.authService.RefreshToken(c.Context(), refreshTokenString)
	if err != nil {
		appLogger.Warn("AuthService failed to refresh token", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(middleware.ErrorResponse{
			Code: "INVALID_REFRESH_TOKEN", Message: "Failed to refresh token: " + err.Error(), Status: fiber.StatusUnauthorized,
		})
	}

	// Log successful refresh. If UserID is needed, it has to be extracted from newAccessToken if not returned by RefreshToken.
	// For simplicity, the service layer already logs this with UserID. Handler log can be simpler.
	appLogger.Info("Tokens refreshed successfully via /auth/refresh endpoint")

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}


// Logout handles user logout.
// @Summary Logout user
// @Description Invalidates user's session/tokens (primarily client-side for JWTs unless server-side blacklisting is used).
// @Tags auth
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string "Logout success message"
// @Router /auth/logout [post]
// Note: ApiKeyAuth for logout implies the access token is sent for logging/blacklisting, which is fine.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	appLogger := logger.Get()
	userIDFromContext, ok := c.Locals(middleware.UserIDKey).(string)
	if ok && userIDFromContext != "" {
		appLogger.Info("User logout request", zap.String("userID", userIDFromContext))
	} else {
		appLogger.Info("Logout request received (user not identified from context)")
	}

	// For JWT, logout is primarily client-side (client discards tokens).
	// If refresh tokens are stored in HttpOnly cookies, clear them:
	// c.Cookie(&fiber.Cookie{
	// Name: "refresh_token", // Or whatever name you use
	// Value: "",
	// Expires: time.Now().Add(-time.Hour), // Expire the cookie
	// HTTPOnly: true,
	// Secure: c.Secure(), // Match Secure attribute from when it was set
	// SameSite: "Lax", // Match SameSite attribute
	// Path: "/", // Match Path attribute
	// })
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Logout successful. Please discard your tokens."})
}
