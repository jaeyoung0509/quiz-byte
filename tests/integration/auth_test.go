package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"quiz-byte/internal/domain/models"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/util"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to get a quiz ID for testing submissions
// It directly queries the database for stability, using the global 'db' and 'subCategoryNameToIDMap'.
func getFirstQuizIDForSubCategory(t *testing.T, subCategoryName string) string {
	var quiz models.Quiz
	var subCatID string

	// Try to find the specific subcategory first
	for k, v := range subCategoryNameToIDMap {
		if strings.Contains(k, subCategoryName) { // Loose matching for flexibility
			subCatID = v
			break
		}
	}

	// If specific subcategory not found, try to get any subcategory ID
	if subCatID == "" {
		for _, v_id := range subCategoryNameToIDMap {
			subCatID = v_id
			logInstance.Warn("Specific subcategory not found, falling back to first available", zap.String("fallback_sub_cat_id", subCatID))
			break
		}
	}
	require.NotEmpty(t, subCatID, "No subcategory ID found in subCategoryNameToIDMap to fetch a quiz for test. Map length: %d", len(subCategoryNameToIDMap))

	// Fetch the first quiz for that subcategory
	// Using positional bindvar :1 for Oracle compatibility with sqlx's Get
	err := db.Get(&quiz, "SELECT id, question, sub_category_id FROM quizzes WHERE sub_category_id = :1 FETCH FIRST 1 ROWS ONLY", subCatID)
	require.NoError(t, err, "Failed to get a quiz from DB for testing using sub_category_id: %s", subCatID)
	require.NotEmpty(t, quiz.ID, "Quiz ID fetched from DB is empty")
	return quiz.ID
}

func TestGetMyProfile_Unauthorized(t *testing.T) {
	t.Run("No Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

		var errorResponse middleware.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Equal(t, "MISSING_AUTH_HEADER", errorResponse.Code)
		assert.Equal(t, "Authorization header is missing", errorResponse.Message) // Changed this line
	})

	t.Run("Malformed Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Authorization", "Bearer malformedtoken")
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

		var errorResponse middleware.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_TOKEN", errorResponse.Code)
		assert.Contains(t, errorResponse.Message, "token is malformed:")
	})

	t.Run("Expired Token", func(t *testing.T) {
		testUser := newTestUser()
		_, err := createTestUserDB(db, testUser) // db from main_test.go
		require.NoError(t, err, "Failed to create test user for expired token test")

		expiredTTL := -1 * time.Hour // A token that expired an hour ago
		// Corrected: generateTestJWTToken takes absolute TTL, not negative.
		// To make it expire, we generate with very short TTL and wait.
		shortTTL := 1 * time.Millisecond
		expiredToken, err := generateTestJWTToken(&testUser, "access", shortTTL)
		require.NoError(t, err, "Failed to generate expired token")

		time.Sleep(50 * time.Millisecond) // Wait for the token to surely expire

		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := cloneResponseBody(resp) // Use helper from main_test.go
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, "Body: %s", bodyBytes.String())

		var errorResponse middleware.ErrorResponse
		err = json.NewDecoder(bodyBytes).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_TOKEN", errorResponse.Code)
		assert.Contains(t, errorResponse.Message, "token is expired")
	})

	t.Run("Refresh Token Used As Access Token", func(t *testing.T) {
		testUser := newTestUser()
		_, err := createTestUserDB(db, testUser)
		require.NoError(t, err, "Failed to create test user for refresh token test")

		refreshTokenTTL := cfg.Auth.JWT.RefreshTokenTTL // Use configured TTL
		refreshToken, err := generateTestJWTToken(&testUser, "refresh", refreshTokenTTL)
		require.NoError(t, err, "Failed to generate refresh token")

		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := cloneResponseBody(resp)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode, "Body: %s", bodyBytes.String())

		var errorResponse middleware.ErrorResponse
		err = json.NewDecoder(bodyBytes).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_TOKEN_TYPE", errorResponse.Code)
		assert.Equal(t, "Invalid token type: expected access, got refresh", errorResponse.Message)
	})

	// Optional: Invalid Signature Token
	// This is harder to reliably test without more direct access to token generation internals
	// or modifying the secret key just for this test case, which might be complex in current setup.
	// For now, we'll skip it as per "Optional - if easy to craft".
}

func TestGetMyProfile_Success(t *testing.T) {
	userID := util.NewULID()
	testUser := models.User{
		ID:        userID,
		Email:     "testuser-" + userID + "@success.com",
		GoogleID:  sql.NullString{String: "googleid-" + userID, Valid: true},
		Name:      sql.NullString{String: "Test User " + userID, Valid: true},
		Picture:   sql.NullString{String: "http://example.com/pic-" + userID + ".jpg", Valid: true},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	createdUser, err := createTestUserDB(db, testUser) // db from main_test.go
	require.NoError(t, err, "Failed to create test user for success profile test")
	require.NotNil(t, createdUser, "Created user should not be nil")

	accessTokenTTL := cfg.Auth.JWT.AccessTokenTTL // Use configured TTL
	accessToken, err := generateTestJWTToken(createdUser, "access", accessTokenTTL)
	require.NoError(t, err, "Failed to generate access token")

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req) // app from main_test.go
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, _ := cloneResponseBody(resp) // Use helper from main_test.go
	fmt.Printf("Raw response body for successful /me: %s\n", bodyBytes.String()) // Debugging line

	require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", bodyBytes.String())

	var userProfileResponse dto.UserProfileResponse
	err = json.NewDecoder(bodyBytes).Decode(&userProfileResponse)
	require.NoError(t, err, "Failed to decode successful /me response. Body: %s", bodyBytes.String())

	assert.Equal(t, createdUser.ID, userProfileResponse.ID)
	assert.Equal(t, createdUser.Email, userProfileResponse.Email)
	assert.Equal(t, createdUser.Name.String, userProfileResponse.Name)
	assert.Equal(t, createdUser.Picture.String, userProfileResponse.Picture)
}

func TestGetMyAttempts_NoAttempts(t *testing.T) {
	// Create a new unique test user
	userID := util.NewULID()
	testUser, err := createTestUserDB(db, models.User{
		ID:        userID,
		Email:     "noattemptsuser-" + userID + "@example.com",
		GoogleID:  sql.NullString{String: "google-noattempts-" + userID, Valid: true},
		Name:      sql.NullString{String: "No Attempts User " + userID, Valid: true},
		Picture:   sql.NullString{String: "http://example.com/noattempts-" + userID + ".jpg", Valid: true},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, testUser)

	// Generate an access token
	accessToken, err := generateTestJWTToken(testUser, "access", cfg.Auth.JWT.AccessTokenTTL)
	require.NoError(t, err)

	// Make GET /users/me/attempts request
	req := httptest.NewRequest(http.MethodGet, "/users/me/attempts", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert status fiber.StatusOK
	bodyBytes, _ := cloneResponseBody(resp)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Body: %s", bodyBytes.String())

	// Decode into dto.UserQuizAttemptsResponse
	var response dto.UserQuizAttemptsResponse
	err = json.NewDecoder(bodyBytes).Decode(&response)
	require.NoError(t, err, "Failed to decode /users/me/attempts response. Body: %s", bodyBytes.String())

	// Assert len(response.Attempts) == 0 and response.PaginationInfo.TotalItems == 0
	assert.Len(t, response.Attempts, 0, "Expected no attempts for a new user")
	assert.Equal(t, int64(0), response.PaginationInfo.TotalItems, "Expected total items to be 0 for a new user")
	assert.Equal(t, 1, response.PaginationInfo.Page, "Page should be 1 for initial request")
}

func TestGetMyAttempts_WithAttempts(t *testing.T) {
	// Setup: Create a unique test user
	userID := util.NewULID()
	userWithAttempts, err := createTestUserDB(db, models.User{
		ID:        userID,
		Email:     "attemptsuser-" + userID + "@example.com",
		GoogleID:  sql.NullString{String: "google-attempts-" + userID, Valid: true},
		Name:      sql.NullString{String: "Attempts User " + userID, Valid: true},
		Picture:   sql.NullString{String: "http://example.com/attempts-" + userID + ".jpg", Valid: true},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, userWithAttempts)

	// Generate an access token for userWithAttempts
	accessToken, err := generateTestJWTToken(userWithAttempts, "access", cfg.Auth.JWT.AccessTokenTTL)
	require.NoError(t, err)

	// Submit 1 or 2 answers for this user
	// For simplicity, we'll use a known subcategory from quiz.json, e.g., "Software Engineering Principles"
	// If subCategoryNameToIDMap is empty, this will fail. Ensure seeding is done.
	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 {
		t.Skip("Skipping TestGetMyAttempts_WithAttempts: no subcategories seeded for subCategoryNameToIDMap.")
		return
	}
	for key := range subCategoryNameToIDMap { // Get any available subcategory name
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1] // parts[1] is the subcategory name itself
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "Could not find a subcategory name from subCategoryNameToIDMap")

	quiz1ID := getFirstQuizIDForSubCategory(t, testSubCategoryName)
	answer1 := "First attempt by user " + userID

	checkAnswerReqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: quiz1ID, UserAnswer: answer1})
	submitReq := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(checkAnswerReqBody))
	submitReq.Header.Set("Authorization", "Bearer "+accessToken)
	submitReq.Header.Set("Content-Type", "application/json")
	submitResp, err := app.Test(submitReq, 60000)
	require.NoError(t, err)
	defer submitResp.Body.Close()
	require.Equal(t, fiber.StatusOK, submitResp.StatusCode, "Failed to submit first answer")

	// Read and decode the check answer response to get the score for later comparison
	submitRespBodyBytes, _ := cloneResponseBody(submitResp)
	var checkResp1 dto.CheckAnswerResponse
	err = json.Unmarshal(submitRespBodyBytes, &checkResp1)
	require.NoError(t, err, "Failed to decode check answer response for first attempt")


	// Optional: Submit a second answer
	quiz2ID := getFirstQuizIDForSubCategory(t, testSubCategoryName) // Could be same quiz or different
	// To ensure it's a *different* quiz, we might need more sophisticated quiz fetching.
	// For now, let's assume it could be the same, or rely on randomness if /api/quiz was used.
	// If getFirstQuizIDForSubCategory always returns the same quiz, this will be two attempts for the SAME quiz.
	// This is acceptable for testing "my attempts" functionality.
	answer2 := "Second attempt by user " + userID
	checkAnswerReqBody2, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: quiz2ID, UserAnswer: answer2})
	submitReq2 := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(checkAnswerReqBody2))
	submitReq2.Header.Set("Authorization", "Bearer "+accessToken)
	submitReq2.Header.Set("Content-Type", "application/json")
	submitResp2, err := app.Test(submitReq2, 60000)
	require.NoError(t, err)
	defer submitResp2.Body.Close()
	require.Equal(t, fiber.StatusOK, submitResp2.StatusCode, "Failed to submit second answer")

	submitRespBodyBytes2, _ := cloneResponseBody(submitResp2)
	var checkResp2 dto.CheckAnswerResponse
	err = json.Unmarshal(submitRespBodyBytes2, &checkResp2)
	require.NoError(t, err, "Failed to decode check answer response for second attempt")


	// Allow time for async recording
	time.Sleep(300 * time.Millisecond) // Increased for safety

	// Action: Make GET /users/me/attempts request
	attemptsReq := httptest.NewRequest(http.MethodGet, "/users/me/attempts?page=1&page_size=5", nil)
	attemptsReq.Header.Set("Authorization", "Bearer "+accessToken)
	attemptsResp, err := app.Test(attemptsReq)
	require.NoError(t, err)
	defer attemptsResp.Body.Close()

	// Assertions
	attemptsRespBodyBytes, _ := cloneResponseBody(attemptsResp)
	require.Equal(t, fiber.StatusOK, attemptsResp.StatusCode, "Body: %s", attemptsRespBodyBytes.String())

	var response dto.UserQuizAttemptsResponse
	err = json.Unmarshal(attemptsRespBodyBytes, &response)
	require.NoError(t, err, "Failed to decode /users/me/attempts response. Body: %s", attemptsRespBodyBytes.String())

	assert.Equal(t, int64(2), response.PaginationInfo.TotalItems, "Expected 2 total attempts")
	assert.Len(t, response.Attempts, 2, "Expected 2 attempts in the response list")

	// Verify details of the attempts. Attempts are ordered by attempted_at DESC.
	// So, response.Attempts[0] should be the second attempt, response.Attempts[1] the first.

	// Second attempt (most recent)
	assert.Equal(t, quiz2ID, response.Attempts[0].QuizID)
	assert.Equal(t, answer2, response.Attempts[0].UserAnswer)
	assert.InDelta(t, checkResp2.Score, response.Attempts[0].LlmScore, 0.001, "LLM score for second attempt mismatch")
	assert.NotEmpty(t, response.Attempts[0].AttemptedAt, "AttemptedAt should not be empty for second attempt")
	assert.Equal(t, checkResp2.Explanation, response.Attempts[0].LlmExplanation)


	// First attempt
	assert.Equal(t, quiz1ID, response.Attempts[1].QuizID)
	assert.Equal(t, answer1, response.Attempts[1].UserAnswer)
	assert.InDelta(t, checkResp1.Score, response.Attempts[1].LlmScore, 0.001, "LLM score for first attempt mismatch")
	assert.NotEmpty(t, response.Attempts[1].AttemptedAt, "AttemptedAt should not be empty for first attempt")
	assert.Equal(t, checkResp1.Explanation, response.Attempts[1].LlmExplanation)
}
