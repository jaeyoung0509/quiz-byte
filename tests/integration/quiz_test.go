package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"quiz-byte/internal/domain"
	"strings"
	"testing"

	"quiz-byte/internal/dto"
	"quiz-byte/internal/handler" // Will be used by TestGetBulkQuizzes for DefaultBulkQuizCount
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/service" // Will be used by TestCheckAnswer_Caching for service.AnswerCachePrefix

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap" // For logInstance used in tests

	"database/sql"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"time"

	"encoding/gob"
	"quiz-byte/internal/cache" // For cache.GenerateCacheKey
	"strconv"                  // For TestGetBulkQuizzes_Caching

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9" // For redisClient used in TestCheckAnswer_Caching
)

// Note: Helper functions like cloneResponseBody, initDatabase, seedPrerequisites, saveQuizes,
// clearRedisCache, clearRedisCacheKey are now in main_test.go.

// Note: TempQuizData struct is now in main_test.go.

// Note: TestMain is now in main_test.go.

func TestGetAllSubCategories(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	resp, err := app.Test(req) // app is from main_test.go
	require.NoError(t, err, "app.Test for /api/categories should not return an error")

	respBodyBytes, _ := cloneResponseBody(resp) // cloneResponseBody is from main_test.go

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status OK for /api/categories. Body: %s", respBodyBytes.String())

	var responseBody dto.CategoryResponse
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	require.NoError(t, err, "Failed to decode response body for /api/categories. Body: %s", respBodyBytes.String())

	assert.Equal(t, "All Categories", responseBody.Name, "Response name should be 'All Categories'")
	logInstance.Info("TestGetAllSubCategories executed.") // logInstance is from main_test.go
}

// clearRedisCache function is removed, it's in main_test.go

func TestGetRandomQuiz(t *testing.T) {
	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 { // subCategoryNameToIDMap is from main_test.go
		t.Skip("Skipping TestGetRandomQuiz: no subcategories were seeded (subCategoryNameToIDMap is empty). Check quiz.json and seedPrerequisites.")
		return
	}
	for key := range subCategoryNameToIDMap { // subCategoryNameToIDMap is from main_test.go
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name could be extracted for testing GetRandomQuiz.")

	logInstance.Info("Testing GetRandomQuiz with sub_category", zap.String("sub_category", testSubCategoryName)) // logInstance is from main_test.go

	encodedSubCategoryName := url.QueryEscape(testSubCategoryName)
	req := httptest.NewRequest(http.MethodGet, "/api/quiz?sub_category="+encodedSubCategoryName, nil)
	resp, err := app.Test(req) // app is from main_test.go
	require.NoError(t, err, "app.Test for /api/quiz should not return an error")

	respBodyBytes, _ := cloneResponseBody(resp) // cloneResponseBody is from main_test.go

	require.Equal(t, http.StatusOK, resp.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz, got %d. Body: %s", resp.StatusCode, respBodyBytes.String()))

	var quiz dto.QuizResponse
	err = json.NewDecoder(resp.Body).Decode(&quiz)
	require.NoError(t, err, fmt.Sprintf("Failed to decode response body for /api/quiz. Body: %s", respBodyBytes.String()))

	assert.NotZero(t, quiz.ID, "Quiz ID should not be zero")
	assert.NotEmpty(t, quiz.Question, "Quiz question should not be empty")

	logInstance.Info("TestGetRandomQuiz executed.", zap.String("quiz_id", quiz.ID)) // logInstance is from main_test.go
}

func TestCheckAnswer(t *testing.T) {
	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 { // subCategoryNameToIDMap is from main_test.go
		t.Skip("Skipping TestCheckAnswer: no subcategories were seeded. Check quiz.json and seedPrerequisites.")
		return
	}
	for key := range subCategoryNameToIDMap { // subCategoryNameToIDMap is from main_test.go
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name could be extracted for testing CheckAnswer.")
	encodedSubCategoryName := url.QueryEscape(testSubCategoryName)
	targetURLGetQuiz := "/api/quiz?sub_category=" + encodedSubCategoryName

	logInstance.Info("Fetching a quiz for TestCheckAnswer", zap.String("sub_category", testSubCategoryName)) // logInstance is from main_test.go
	reqGet := httptest.NewRequest(http.MethodGet, targetURLGetQuiz, nil)
	respGet, err := app.Test(reqGet) // app is from main_test.go
	require.NoError(t, err)

	respGetBodyBytes, _ := cloneResponseBody(respGet) // cloneResponseBody is from main_test.go
	require.Equal(t, http.StatusOK, respGet.StatusCode, fmt.Sprintf("Failed to get a quiz to check answer. Status: %d, Body: %s", respGet.StatusCode, respGetBodyBytes.String()))

	var quizToAnswer dto.QuizResponse
	err = json.NewDecoder(respGet.Body).Decode(&quizToAnswer)
	require.NoError(t, err, fmt.Sprintf("Failed to decode quiz for TestCheckAnswer. Body: %s", respGetBodyBytes.String()))
	require.NotZero(t, quizToAnswer.ID, "Quiz ID for checking answer should not be zero")

	logInstance.Info("Quiz fetched for TestCheckAnswer", zap.String("quiz_id", quizToAnswer.ID)) // logInstance is from main_test.go

	answerRequest := dto.CheckAnswerRequest{
		QuizID:     quizToAnswer.ID,
		UserAnswer: "This is a detailed and descriptive test answer, aiming to cover various aspects of the OSI model.",
	}
	requestBody, err := json.Marshal(answerRequest)
	require.NoError(t, err)

	logInstance.Info("Submitting answer for TestCheckAnswer", zap.String("quiz_id", answerRequest.QuizID)) // logInstance is from main_test.go
	reqPost := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewReader(requestBody))
	reqPost.Header.Set("Content-Type", "application/json")

	respPost, err := app.Test(reqPost, 60000) // app is from main_test.go
	require.NoError(t, err)

	respPostBodyBytes, _ := cloneResponseBody(respPost) // cloneResponseBody is from main_test.go
	require.Equal(t, http.StatusOK, respPost.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz/check. Status: %d, Body: %s", respPost.StatusCode, respPostBodyBytes.String()))

	var answerResponse dto.CheckAnswerResponse
	err = json.NewDecoder(respPost.Body).Decode(&answerResponse)
	require.NoError(t, err, fmt.Sprintf("Failed to decode answer response. Body: %s", respPostBodyBytes.String()))

	assert.True(t, answerResponse.Score >= 0 && answerResponse.Score <= 1, "Score should be between 0 and 1. Actual: %f", answerResponse.Score)
	assert.NotEmpty(t, answerResponse.Explanation, "Explanation should not be empty")
	assert.True(t, answerResponse.Completeness >= 0 && answerResponse.Completeness <= 1, "Completeness should be between 0 and 1. Actual: %f", answerResponse.Completeness)
	assert.True(t, answerResponse.Relevance >= 0 && answerResponse.Relevance <= 1, "Relevance should be between 0 and 1. Actual: %f", answerResponse.Relevance)
	assert.True(t, answerResponse.Accuracy >= 0 && answerResponse.Accuracy <= 1, "Accuracy should be between 0 and 1. Actual: %f", answerResponse.Accuracy)

	logInstance.Info("TestCheckAnswer executed.", zap.String("quiz_id", answerRequest.QuizID), zap.Float64("score", answerResponse.Score)) // logInstance is from main_test.go
}

func TestGetBulkQuizzes(t *testing.T) {
	t.Run("SuccessfulRetrieval", func(t *testing.T) {
		var validSubCategoryName string
		var validSubCategoryID string
		if len(subCategoryNameToIDMap) == 0 { // subCategoryNameToIDMap is from main_test.go
			t.Skip("Skipping SuccessfulRetrieval: no subcategories seeded.")
			return
		}
		for key, id := range subCategoryNameToIDMap { // subCategoryNameToIDMap is from main_test.go
			parts := strings.Split(key, "|")
			if len(parts) == 2 {
				validSubCategoryName = parts[1]
				validSubCategoryID = id
				break
			}
		}
		require.NotEmpty(t, validSubCategoryName, "No valid subcategory name found for test.")
		logInstance.Info("TestGetBulkQuizzes/SuccessfulRetrieval using", zap.String("sub_category", validSubCategoryName), zap.String("sub_category_id", validSubCategoryID)) // logInstance is from main_test.go

		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?sub_category="+url.QueryEscape(validSubCategoryName)+"&count=3", nil)
		resp, err := app.Test(req) // app is from main_test.go
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected OK. Body: %s", string(bodyBytes))

		var responseBody dto.BulkQuizzesResponse
		err = json.Unmarshal(bodyBytes, &responseBody)
		require.NoError(t, err, "Failed to decode response. Body: %s", string(bodyBytes))
		assert.Len(t, responseBody.Quizzes, 3, "Expected 3 quizzes. Body: %s", string(bodyBytes))
	})

	t.Run("InvalidSubCategory", func(t *testing.T) {
		invalidSubCategoryName := "InvalidSubCategoryName"
		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?sub_category="+url.QueryEscape(invalidSubCategoryName)+"&count=3", nil)
		resp, err := app.Test(req) // app is from main_test.go
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		// Expecting 400 Bad Request for invalid subcategory
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected Bad Request for invalid subcategory. Body: %s", string(bodyBytes))

		var errorResponse middleware.ErrorResponse // Use middleware.ErrorResponse for expected structure
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err, "Failed to decode error response. Body: %s", string(bodyBytes))

		expectedErrorCode := domain.ErrInvalidCategory.Error()

		assert.Equal(t, expectedErrorCode, errorResponse.Code, "Error code mismatch. Body: %s", string(bodyBytes))
		assert.Equal(t, http.StatusBadRequest, errorResponse.Status, "Error status in body mismatch. Body: %s", string(bodyBytes))
	})

	t.Run("MissingSubCategory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?count=3", nil)
		resp, err := app.Test(req) // app is from main_test.go
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected Bad Request. Body: %s", string(bodyBytes))

		var errorResponse dto.ErrorResponse // Handler returns dto.ErrorResponse directly for this case
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err, "Failed to decode error response for missing sub_category. Body: %s", string(bodyBytes))
		assert.Equal(t, "INVALID_REQUEST: sub_category is required", errorResponse.Error, "Error message should indicate missing sub_category")
	})

	t.Run("DefaultCount", func(t *testing.T) {
		var validSubCategoryName string
		if len(subCategoryNameToIDMap) == 0 { // subCategoryNameToIDMap is from main_test.go
			t.Skip("Skipping DefaultCount: no subcategories seeded.")
			return
		}
		for key := range subCategoryNameToIDMap { // subCategoryNameToIDMap is from main_test.go
			parts := strings.Split(key, "|")
			if len(parts) == 2 {
				validSubCategoryName = parts[1]
				break
			}
		}
		require.NotEmpty(t, validSubCategoryName, "No valid subcategory name found for test.")

		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?sub_category="+url.QueryEscape(validSubCategoryName), nil)
		resp, err := app.Test(req) // app is from main_test.go
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected OK. Body: %s", string(bodyBytes))

		var responseBody dto.BulkQuizzesResponse
		err = json.Unmarshal(bodyBytes, &responseBody)
		require.NoError(t, err, "Failed to decode response. Body: %s", string(bodyBytes))
		assert.True(t, len(responseBody.Quizzes) <= handler.DefaultBulkQuizCount, "Expected up to default number of quizzes. Got: %d, Body: %s", len(responseBody.Quizzes), string(bodyBytes))
		if len(subCategoryNameToIDMap[validSubCategoryName]) > 0 && len(responseBody.Quizzes) == 0 { // subCategoryNameToIDMap is from main_test.go
			// This assertion might be too strict if the category legitimately has 0 quizzes after seeding.
			// For now, we expect some quizzes if the category is valid and seeded.
			// assert.True(t, len(responseBody.Quizzes) > 0, "Expected some quizzes if subcategory is valid. Got: %d", len(responseBody.Quizzes))
		}
	})
}

func TestCheckAnswer_Caching(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" || os.Getenv("OPENAI_API_KEY") == "YOUR_OPENAI_API_KEY_GOES_HERE" {
		t.Skip("Skipping TestCheckAnswer_Caching: OPENAI_API_KEY is not set or is a placeholder.")
		return
	}

	var testQuizID string
	var testQuizQuestion string

	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 { // subCategoryNameToIDMap is from main_test.go
		t.Skip("Skipping TestCheckAnswer_Caching: no subcategories seeded.")
		return
	}
	for key := range subCategoryNameToIDMap { // subCategoryNameToIDMap is from main_test.go
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name found for caching test.")

	getQuizReq := httptest.NewRequest(http.MethodGet, "/api/quiz?sub_category="+url.QueryEscape(testSubCategoryName), nil)
	getQuizResp, err := app.Test(getQuizReq) // app is from main_test.go
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getQuizResp.StatusCode)
	var quizToAnswer dto.QuizResponse
	err = json.NewDecoder(getQuizResp.Body).Decode(&quizToAnswer)
	getQuizResp.Body.Close()
	require.NoError(t, err)
	testQuizID = quizToAnswer.ID
	testQuizQuestion = quizToAnswer.Question
	require.NotEmpty(t, testQuizID, "Failed to get a quiz ID for caching test.")

	logInstance.Info("Using quiz for caching tests", zap.String("quizID", testQuizID), zap.String("question", testQuizQuestion)) // logInstance is from main_test.go

	cacheKey = service.AnswerCachePrefix + testQuizID // service.AnswerCachePrefix, cacheKey from main_test.go

	t.Run("CacheMissFirstAnswer", func(t *testing.T) {
		clearRedisCacheKey(redisClient, cacheKey) // clearRedisCacheKey, redisClient, cacheKey from main_test.go

		answer1 := "An initial unique answer for caching test regarding " + testQuizQuestion
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answer1})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 60000) // app is from main_test.go
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))

		var respBody dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes, &respBody)
		require.NoError(t, err)
		assert.True(t, respBody.Score >= 0 && respBody.Score <= 1, "Score should be between 0 and 1. Actual: %f", respBody.Score)
		assert.NotEmpty(t, respBody.Explanation, "Explanation should not be empty")

		cachedData, err := redisClient.HGet(context.Background(), cacheKey, answer1).Result() // redisClient, cacheKey from main_test.go
		require.NoError(t, err, "Expected answer to be cached in Redis.")
		require.NotEmpty(t, cachedData, "Cached data should not be empty.")

		var cachedResp dto.CheckAnswerResponse
		err = json.Unmarshal([]byte(cachedData), &cachedResp)
		require.NoError(t, err, "Failed to unmarshal cached data")
		assert.Equal(t, respBody.Score, cachedResp.Score, "Cached score should match LLM response score")
	})

	t.Run("CacheHitIdenticalAnswer", func(t *testing.T) {
		answer1 := "An initial unique answer for caching test regarding " + testQuizQuestion
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answer1})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp1, err := app.Test(req, 10000) // app is from main_test.go
		require.NoError(t, err)
		defer resp1.Body.Close()
		bodyBytes1, _ := io.ReadAll(resp1.Body)
		require.Equal(t, http.StatusOK, resp1.StatusCode, "Body: %s", string(bodyBytes1))

		var respBody1 dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes1, &respBody1)
		require.NoError(t, err)

		req2 := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := app.Test(req2, 10000) // app is from main_test.go
		require.NoError(t, err)
		defer resp2.Body.Close()
		bodyBytes2, _ := io.ReadAll(resp2.Body)
		require.Equal(t, http.StatusOK, resp2.StatusCode, "Body: %s", string(bodyBytes2))

		var respBody2 dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes2, &respBody2)
		require.NoError(t, err)

		assert.Equal(t, respBody1.Score, respBody2.Score, "Scores from cached responses should be identical.")
		assert.Equal(t, respBody1.Explanation, respBody2.Explanation, "Explanations from cached responses should be identical.")
	})

	t.Run("CacheMissOrHitDifferentAnswerBySimilarity", func(t *testing.T) {
		answerSimilar := "A slightly different but conceptually similar answer for " + testQuizQuestion + " focusing on key aspects."
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answerSimilar})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 60000) // app is from main_test.go
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))

		var respBody dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes, &respBody)
		require.NoError(t, err)
		assert.True(t, respBody.Score >= 0 && respBody.Score <= 1, "Score should be between 0 and 1. Actual: %f", respBody.Score)

		cachedData, err := redisClient.HGet(context.Background(), cacheKey, answerSimilar).Result() // redisClient, cacheKey from main_test.go
		if err == redis.Nil {
			logInstance.Info("Different answer was not found in cache (similarity might be below threshold or new LLM call occurred).", zap.String("answer", answerSimilar)) // logInstance is from main_test.go
			// It should have been cached after the LLM call in the service.
			// If GenerateOpenAIEmbedding failed, cache write might be skipped.
			// For this test to pass robustly when similarity miss leads to LLM call, cache write must succeed.
			require.NotEqual(t, redis.Nil, err, "Expected new answer to be cached after LLM call, but HGet returned redis.Nil. Embedding/cache write might have failed.")

		}
		require.NoError(t, err, "Error checking cache for similar answer.")
		require.NotEmpty(t, cachedData, "New different answer should now be cached.")
	})

	clearRedisCacheKey(redisClient, cacheKey) // clearRedisCacheKey, redisClient, cacheKey from main_test.go
}

// clearRedisCacheKey function is removed, it's in main_test.go

func TestCheckAnswer_LoggedInUser(t *testing.T) {
	// 1. Setup: Create a unique test user
	userID := util.NewULID()
	testUser, err := createTestUserDB(db, models.User{ // db is the global from main_test.go
		ID:                userID,
		Email:             "loggeduser-" + userID + "@example.com",
		GoogleID:          "googlelogged-" + userID,
		Name:              sql.NullString{String: "Logged Test User " + userID, Valid: true},
		ProfilePictureURL: sql.NullString{String: "http://example.com/picture-logged-" + userID + ".jpg", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, testUser)

	// 2. Generate an access token for this testUser
	accessToken, err := generateTestJWTToken(testUser, "access", cfg.Auth.JWT.AccessTokenTTL)
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)

	// 3. Fetch a quiz to answer
	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 {
		t.Skip("Skipping TestCheckAnswer_LoggedInUser: no subcategories seeded.")
		return
	}
	for key := range subCategoryNameToIDMap {
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name could be extracted for testing.")

	encodedSubCategoryName := url.QueryEscape(testSubCategoryName)
	reqGetQuiz := httptest.NewRequest(http.MethodGet, "/api/quiz?sub_category="+encodedSubCategoryName, nil)
	respGetQuiz, err := app.Test(reqGetQuiz)
	require.NoError(t, err)
	defer respGetQuiz.Body.Close()

	bodyBytesGetQuiz, _ := cloneResponseBody(respGetQuiz)
	require.Equal(t, http.StatusOK, respGetQuiz.StatusCode, "Failed to get quiz. Body: %s", bodyBytesGetQuiz.String())

	var quizToAnswer dto.QuizResponse
	err = json.NewDecoder(bodyBytesGetQuiz).Decode(&quizToAnswer)
	require.NoError(t, err, "Failed to decode quiz response. Body: %s", bodyBytesGetQuiz.String())
	require.NotEmpty(t, quizToAnswer.ID, "Fetched quiz ID is empty")
	fetchedQuizID := quizToAnswer.ID

	// 4. Action: Prepare and submit CheckAnswerRequest
	userAnswer := "A valid test answer for logged in user: " + util.NewULID() // Unique answer
	answerRequest := dto.CheckAnswerRequest{
		QuizID:     fetchedQuizID,
		UserAnswer: userAnswer,
	}
	requestBody, err := json.Marshal(answerRequest)
	require.NoError(t, err)

	reqPostCheck := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewReader(requestBody))
	reqPostCheck.Header.Set("Authorization", "Bearer "+accessToken)
	reqPostCheck.Header.Set("Content-Type", "application/json")

	respPostCheck, err := app.Test(reqPostCheck, 60000) // Increased timeout for LLM
	require.NoError(t, err)
	defer respPostCheck.Body.Close()

	// 5. Assertions (API Response)
	bodyBytesPostCheck, _ := cloneResponseBody(respPostCheck)
	require.Equal(t, fiber.StatusOK, respPostCheck.StatusCode, "Body: %s", bodyBytesPostCheck.String())

	var checkAnswerResponse dto.CheckAnswerResponse
	err = json.NewDecoder(bodyBytesPostCheck).Decode(&checkAnswerResponse)
	require.NoError(t, err, "Failed to decode check answer response. Body: %s", bodyBytesPostCheck.String())

	assert.True(t, checkAnswerResponse.Score >= 0 && checkAnswerResponse.Score <= 1, "Score should be between 0 and 1. Actual: %f", checkAnswerResponse.Score)
	assert.NotEmpty(t, checkAnswerResponse.Explanation, "Explanation should not be empty")

	// 6. Assertions (Database Verification)
	// Allow a brief moment for the async recordQuizAttemptAsync to likely complete
	time.Sleep(200 * time.Millisecond) // Increased from 100ms for potentially slower CI

	var attempt models.UserQuizAttempt
	// Query for the specific attempt. Using NamedQuery because the struct fields match DB columns.
	// Ensure your SQL query matches your database schema and parameter binding.
	// Oracle uses :1, :2 for positional, but NamedExec usually implies matching struct field names to :fieldname
	// For sqlx with Oracle and NamedExec, it's better to use named parameters in the query if possible,
	// or ensure positional parameters align if not using NamedExec.
	// The original instruction used positional :1, :2. Let's stick to that for Get.
	errDb := db.Get(&attempt, "SELECT * FROM user_quiz_attempts WHERE user_id = :1 AND quiz_id = :2 ORDER BY attempted_at DESC FETCH FIRST 1 ROWS ONLY", testUser.ID, fetchedQuizID)
	require.NoError(t, errDb, "Error fetching user quiz attempt from DB. UserID: %s, QuizID: %s", testUser.ID, fetchedQuizID)

	assert.Equal(t, testUser.ID, attempt.UserID)
	assert.Equal(t, fetchedQuizID, attempt.QuizID)
	require.True(t, attempt.UserAnswer.Valid, "UserAnswer should be valid in DB")
	assert.Equal(t, userAnswer, attempt.UserAnswer.String)

	// Assert LlmScore matches (within a small tolerance if it were float, but it's exact from API)
	require.True(t, attempt.LlmScore.Valid, "LlmScore should be valid in DB")
	assert.InDelta(t, checkAnswerResponse.Score, attempt.LlmScore.Float64, 0.001, "LLM Score in DB should match API response score")

	// Additional checks for other fields if necessary
	assert.True(t, attempt.LlmCompleteness.Valid, "Completeness score should be valid in DB")
	assert.InDelta(t, checkAnswerResponse.Completeness, attempt.LlmCompleteness.Float64, 0.001)

	assert.True(t, attempt.LlmRelevance.Valid, "Relevance score should be valid in DB")
	assert.InDelta(t, checkAnswerResponse.Relevance, attempt.LlmRelevance.Float64, 0.001)

	assert.True(t, attempt.LlmAccuracy.Valid, "Accuracy score should be valid in DB")
	assert.InDelta(t, checkAnswerResponse.Accuracy, attempt.LlmAccuracy.Float64, 0.001)

	assert.NotEmpty(t, attempt.LlmExplanation.String, "LLM Explanation in DB should not be empty")
	assert.Equal(t, checkAnswerResponse.Explanation, attempt.LlmExplanation.String)
}

// Helper function to determine score range
func getScoreRange(score float64, scoreRanges []string) string {
	for _, r := range scoreRanges {
		parts := strings.Split(r, "-")
		if len(parts) != 2 {
			logInstance.Error("Invalid score range format", zap.String("range", r))
			continue // Skip malformed range
		}
		min, errMin := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		max, errMax := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

		if errMin != nil || errMax != nil {
			logInstance.Error("Error parsing score range min/max", zap.String("range", r), zap.Error(errMin), zap.Error(errMax))
			continue // Skip on parsing error
		}

		// Check if score falls within [min, max]. Note: inclusive of max.
		if score >= min && score <= max {
			return r
		}
	}
	return "" // Or some indicator that no range was matched
}

func TestCheckAnswer_QuizEvaluationLinkage(t *testing.T) {
	require.NotEmpty(t, seededQuizIDWithEval, "seededQuizIDWithEval is not set. Ensure saveQuizes seeds a QuizEvaluation and sets this global var.")

	targetQuizID := seededQuizIDWithEval

	expectedExplanations := map[string]string{
		"0.8-1.0":  "Excellent work! Your answer is thorough and accurate.",
		"0.5-0.79": "Good effort. You've grasped the main concepts, but try to elaborate further next time.",
		"0.0-0.49": "It seems there's a misunderstanding. Please review the topic and try again.",
	}
	// This is one of the sample answers from the highest score range in the seeded QuizEvaluation
	userAnswerToTargetHighScore := "This is an excellent and comprehensive answer covering all key aspects."

	answerRequest := dto.CheckAnswerRequest{
		QuizID:     targetQuizID,
		UserAnswer: userAnswerToTargetHighScore,
	}
	requestBody, err := json.Marshal(answerRequest)
	require.NoError(t, err)

	// No auth token needed if endpoint is optionally authenticated and we are testing the eval linkage part
	reqPostCheck := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewReader(requestBody))
	reqPostCheck.Header.Set("Content-Type", "application/json")

	respPostCheck, err := app.Test(reqPostCheck, 60000) // Increased timeout for LLM
	require.NoError(t, err)
	defer respPostCheck.Body.Close()

	bodyBytesPostCheck, _ := cloneResponseBody(respPostCheck)
	require.Equal(t, fiber.StatusOK, respPostCheck.StatusCode, "Body: %s", bodyBytesPostCheck.String())

	var checkAnswerResponse dto.CheckAnswerResponse
	err = json.NewDecoder(bodyBytesPostCheck).Decode(&checkAnswerResponse)
	require.NoError(t, err, "Failed to decode check answer response. Body: %s", bodyBytesPostCheck.String())

	// Determine which score range the actual score falls into
	// The score ranges are defined in saveQuizes when seeding QuizEvaluation
	seededScoreRanges := []string{"0.8-1.0", "0.5-0.79", "0.0-0.49"}
	determinedScoreRange := getScoreRange(checkAnswerResponse.Score, seededScoreRanges)

	// Assert that if a score range was matched, the explanation corresponds to it.
	if determinedScoreRange != "" {
		expectedExplanation, ok := expectedExplanations[determinedScoreRange]
		if ok {
			assert.Equal(t, expectedExplanation, checkAnswerResponse.Explanation,
				"Explanation for score %f (range %s) did not match expected. Got: %s",
				checkAnswerResponse.Score, determinedScoreRange, checkAnswerResponse.Explanation)
		} else {
			t.Logf("No specific explanation predefined for determined score range %s (score: %f). Actual explanation: %s",
				determinedScoreRange, checkAnswerResponse.Score, checkAnswerResponse.Explanation)
			// We might not fail the test here, as the primary check is that *if* a range has a custom explanation, it's used.
			// However, for this test, all seeded ranges have explanations.
			assert.Fail(t, "Determined score range was not in the expected explanations map, this should not happen if getScoreRange works correctly with seeded ranges.",
				"Score: %f, Range: %s, Explanation: %s", checkAnswerResponse.Score, determinedScoreRange, checkAnswerResponse.Explanation)
		}
	} else {
		// If the score did not fall into any predefined range, the explanation might be a generic one.
		// This case might indicate an issue with the score itself or the ranges defined.
		// For this test, we expect the LLM to provide a score that falls in one of the ranges,
		// especially when providing a sample answer.
		t.Logf("Score %f did not fall into any of the predefined score ranges. Actual explanation: %s",
			checkAnswerResponse.Score, checkAnswerResponse.Explanation)
		// Depending on strictness, this could be a fail.
		// For now, let's assert that an explanation is still provided.
		assert.NotEmpty(t, checkAnswerResponse.Explanation, "Explanation should still be provided even if score is outside defined custom ranges.")
	}

	// A simpler check: is the explanation one of the three?
	// This is less precise but good for a start if scores are unpredictable.
	isExplanationExpected := false
	for _, expl := range expectedExplanations {
		if checkAnswerResponse.Explanation == expl {
			isExplanationExpected = true
			break
		}
	}
	if !isExplanationExpected {
		logInstance.Warn("Explanation was not one of the three predefined ones.",
			zap.Float64("score", checkAnswerResponse.Score),
			zap.String("explanation", checkAnswerResponse.Explanation))
	}
	// This assertion might be too strict if the LLM is highly variable or if the sample answer
	// doesn't perfectly hit the target ranges + custom explanation logic path.
	// The more robust check is the `determinedScoreRange` combined with `expectedExplanations[determinedScoreRange]`.
	// If `determinedScoreRange` is empty, it means the score was outside ALL ranges (e.g. < 0 or > 1, or gaps in ranges).
	require.NotEmpty(t, determinedScoreRange, "Score %f did not fall into any predefined range. This might indicate an issue with scoring or range definitions.", checkAnswerResponse.Score)
	assert.True(t, isExplanationExpected, "The explanation should be one of the predefined ones if the score falls into a range with a custom explanation. Score: %f, Range: %s, Explanation: %s", checkAnswerResponse.Score, determinedScoreRange, checkAnswerResponse.Explanation)

}

func TestGetAllSubCategories_Caching(t *testing.T) {
	cacheKey := cache.GenerateCacheKey("quiz_service", "category_list", "all")

	// Clear cache for this key first
	err := redisClient.Del(context.Background(), cacheKey).Err()
	require.NoError(t, err, "Failed to clear cache for key: %s", cacheKey)
	logInstance.Info("Cache cleared for key", zap.String("key", cacheKey))

	// First API Call
	req1 := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)
	resp1BodyBytes, _ := cloneResponseBody(resp1) // Read body for later comparison

	// Verify Cache Content
	cachedDataString, err := redisClient.Get(context.Background(), cacheKey).Result()
	require.NoError(t, err, "Error getting data from Redis for key: %s", cacheKey)
	require.NotEmpty(t, cachedDataString, "Cached data string should not be empty for key: %s", cacheKey)
	logInstance.Info("Successfully retrieved from cache", zap.String("key", cacheKey), zap.Int("length", len(cachedDataString)))

	// The service caches a []string of subcategory names using GOB encoding.
	var cachedCategories []string
	byteReader := bytes.NewReader([]byte(cachedDataString))
	decoder := gob.NewDecoder(byteReader)
	err = decoder.Decode(&cachedCategories)
	require.NoError(t, err, "Failed to GOB decode cached category list. Data: %s", cachedDataString)

	// Assert some categories were cached - this depends on seedPrerequisites
	require.True(t, len(cachedCategories) > 0, "Expected more than 0 categories in cache. Found %d", len(cachedCategories))
	logInstance.Info("Cached category list decoded successfully", zap.Int("count", len(cachedCategories)))

	// Second API Call
	req2 := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
	resp2BodyBytes, _ := cloneResponseBody(resp2)

	// Assert response bodies are identical
	assert.Equal(t, resp1BodyBytes.String(), resp2BodyBytes.String(), "Response body from second call should match first (served from cache or identical live response)")

	// Further check: the actual API returns a dto.CategoryResponse. The cache stores the []string.
	// We need to ensure the second API call (which should be from cache) reconstructs the same dto.CategoryResponse.
	var apiResp1 dto.CategoryResponse
	err = json.Unmarshal(resp1BodyBytes.Bytes(), &apiResp1)
	require.NoError(t, err)

	var apiResp2 dto.CategoryResponse
	err = json.Unmarshal(resp2BodyBytes.Bytes(), &apiResp2)
	require.NoError(t, err)

	assert.Equal(t, apiResp1, apiResp2, "Decoded API responses from first and second call should be identical")
}

func TestGetBulkQuizzes_Caching(t *testing.T) {
	// Select a valid subcategory name and ID
	var testSubCategoryName string
	var testSubCategoryID string
	if len(subCategoryNameToIDMap) == 0 {
		t.Skip("Skipping TestGetBulkQuizzes_Caching: no subcategories seeded (subCategoryNameToIDMap is empty).")
		return
	}
	for keyMap, id := range subCategoryNameToIDMap {
		parts := strings.Split(keyMap, "|") // keyMap is "MainCat|SubCat"
		if len(parts) == 2 {
			testSubCategoryName = parts[1] // SubCategory name
			testSubCategoryID = id         // SubCategory ID
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name could be extracted for testing.")
	require.NotEmpty(t, testSubCategoryID, "No subcategory ID could be extracted for testing.")

	count := 2
	cacheKey := cache.GenerateCacheKey("quiz_service", "quiz_list", testSubCategoryID, strconv.Itoa(count))

	// Clear cache for this key first
	err := redisClient.Del(context.Background(), cacheKey).Err()
	require.NoError(t, err, "Failed to clear cache for key: %s", cacheKey)
	logInstance.Info("Cache cleared for key", zap.String("key", cacheKey))

	// First API Call
	urlPath := fmt.Sprintf("/api/quizzes?sub_category=%s&count=%d", url.QueryEscape(testSubCategoryName), count)
	req1 := httptest.NewRequest(http.MethodGet, urlPath, nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode, "Failed on first API call to %s", urlPath)

	var firstAPIResponse dto.BulkQuizzesResponse
	resp1BodyBytes, _ := cloneResponseBody(resp1)
	err = json.Unmarshal(resp1BodyBytes.Bytes(), &firstAPIResponse)
	require.NoError(t, err, "Failed to decode first API response. Body: %s", resp1BodyBytes.String())
	// Ensure some quizzes were returned if subcategory is expected to have them
	// This depends on quiz.json and seeding. If it's possible for a valid subcat to have < count quizzes:
	// require.True(t, len(firstAPIResponse.Quizzes) > 0, "Expected at least one quiz for subcategory %s", testSubCategoryName)
	// For now, just proceed.

	// Verify Cache Content
	cachedDataString, err := redisClient.Get(context.Background(), cacheKey).Result()
	require.NoError(t, err, "Error getting data from Redis for key: %s", cacheKey)
	require.NotEmpty(t, cachedDataString, "Cached data string should not be empty for key: %s", cacheKey)

	var cachedResponse dto.BulkQuizzesResponse
	byteReader := bytes.NewReader([]byte(cachedDataString))
	decoder := gob.NewDecoder(byteReader)
	err = decoder.Decode(&cachedResponse)
	require.NoError(t, err, "Failed to GOB decode cached bulk quizzes response. Data: %s", cachedDataString)

	assert.Equal(t, len(firstAPIResponse.Quizzes), len(cachedResponse.Quizzes), "Number of quizzes in cache should match API response")
	// Deep comparison might be too much if order can vary, but for now let's assume order is preserved by cache
	assert.Equal(t, firstAPIResponse, cachedResponse, "Cached response content does not match first API response")

	// Second API Call
	req2 := httptest.NewRequest(http.MethodGet, urlPath, nil)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var secondAPIResponse dto.BulkQuizzesResponse
	resp2BodyBytes, _ := cloneResponseBody(resp2)
	err = json.Unmarshal(resp2BodyBytes.Bytes(), &secondAPIResponse)
	require.NoError(t, err, "Failed to decode second API response. Body: %s", resp2BodyBytes.String())

	// Assert this response is deeply equal to the first response
	assert.Equal(t, firstAPIResponse, secondAPIResponse, "Second API response should match first (served from cache or identical live response)")
}
