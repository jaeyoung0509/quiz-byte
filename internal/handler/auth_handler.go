package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"quiz-byte/internal/dto"        // Ensure dto is imported if AuthenticatedUser is used explicitly in handler (it is for logging)
	"quiz-byte/internal/logger"     // Added
	"quiz-byte/internal/middleware" // For middleware.ErrorResponse
	"quiz-byte/internal/service"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap" // Added
)

const (
	oauthStateCookieName = "oauthstate"
	// Note: Frontend URL should come from config for redirects
)

type AuthHandler struct {
	authService service.AuthService
	// appConfig   *config.Config // Removed
}

func NewAuthHandler(authService service.AuthService /*, appConfig *config.Config */) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		// appConfig:   appConfig, // Removed
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

	accessToken, refreshToken, authUser, err := h.authService.HandleGoogleCallback(c.Context(), code, receivedState, expectedState) // authUser is now *dto.AuthenticatedUser
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

	if authUser != nil { // Check authUser
		appLogger.Info("Google OAuth callback successful, tokens issued", zap.String("userID", authUser.ID)) // Use authUser.ID
	} else {
		appLogger.Error("AuthenticatedUser object is nil after successful Google OAuth callback")
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
	return c.Status(fiber.StatusOK).JSON(dto.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
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
	var req dto.RefreshTokenRequest // Changed to dto.RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		appLogger.Warn("Failed to parse request body for token refresh", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "INVALID_REQUEST_BODY", Message: "Invalid request body", Status: fiber.StatusBadRequest,
		})
	}

	// TODO: Add validation for req.RefreshToken if not handled by a global validator
	// Example: if req.RefreshToken == "" { ... }

	if req.RefreshToken == "" { // Basic validation
		appLogger.Warn("Refresh token missing in request body")
		return c.Status(fiber.StatusBadRequest).JSON(middleware.ErrorResponse{
			Code: "MISSING_REFRESH_TOKEN", Message: "Refresh token is missing in request body", Status: fiber.StatusBadRequest,
		})
	}

	newAccessToken, newRefreshToken, err := h.authService.RefreshToken(c.Context(), req.RefreshToken) // Use req.RefreshToken
	if err != nil {
		appLogger.Warn("AuthService failed to refresh token", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(middleware.ErrorResponse{
			Code: "INVALID_REFRESH_TOKEN", Message: "Failed to refresh token: " + err.Error(), Status: fiber.StatusUnauthorized,
		})
	}

	// Log successful refresh. If UserID is needed, it has to be extracted from newAccessToken if not returned by RefreshToken.
	// For simplicity, the service layer already logs this with UserID. Handler log can be simpler.
	appLogger.Info("Tokens refreshed successfully via /auth/refresh endpoint")

	return c.Status(fiber.StatusOK).JSON(dto.TokenResponse{ // Changed to dto.TokenResponse
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
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
	return c.Status(fiber.StatusOK).JSON(dto.MessageResponse{Message: "Logout successful. Please discard your tokens."}) // Changed to dto.MessageResponse
}
