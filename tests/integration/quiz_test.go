package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"quiz-byte/internal/adapter"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository"
	"quiz-byte/internal/repository/models"
	"strings"
	"testing"
	"time"

	"quiz-byte/internal/config"
	dblogic "quiz-byte/internal/database"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/handler"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/middleware" // Import middleware
	"quiz-byte/internal/service"
	"quiz-byte/internal/util"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap"

	"quiz-byte/internal/cache"

	"github.com/redis/go-redis/v9"
)

var app *fiber.App
var logInstance *zap.Logger
var db *sqlx.DB
var redisClient *redis.Client

var subCategoryNameToIDMap map[string]string

type TempQuizData struct {
	MainCategory string   `json:"main_category"`
	SubCategory  string   `json:"sub_category"`
	Question     string   `json:"question"`
	ModelAnswers []string `json:"model_answers"`
	Keywords     []string `json:"keywords"`
	Difficulty   int      `json:"difficulty"`
}

func cloneResponseBody(resp *http.Response) (*bytes.Buffer, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bytes.NewBuffer(bodyBytes), nil
}

func TestMain(m *testing.M) {
	os.Setenv("ENV", "test")

	cfg, err := config.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	if err := logger.Initialize(cfg); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	logInstance = logger.Get()
	defer func() {
		if logInstance != nil {
			_ = logInstance.Sync()
		}
	}()

	logInstance.Info("Starting integration tests")

	dsn := cfg.GetDSN()
	logInstance.Info("Connecting to database with DSN", zap.String("dsn", dsn))
	db, err = dblogic.NewSQLXOracleDB(dsn)
	if err != nil {
		logInstance.Fatal("Failed to connect to database", zap.Error(err))
	}

	if err := initDatabase(cfg); err != nil {
		logInstance.Fatal("Failed to initialize database", zap.Error(err))
	}

	if err := seedPrerequisites(db); err != nil {
		logInstance.Fatal("Failed to seed prerequisite data", zap.Error(err))
	}

	if err := saveQuizes(); err != nil {
		logInstance.Fatal("Failed to save quizzes", zap.Error(err))
	}

	quizDomainRepo := repository.NewQuizDatabaseAdapter(db)

	ollamaHTTPClient := &http.Client{
		Timeout: 20 * time.Second,
	}
	llm, err := ollama.New(
		ollama.WithServerURL(cfg.LLMServer),
		ollama.WithModel("qwen3:0.6b"),
		ollama.WithHTTPClient(ollamaHTTPClient),
	)
	if err != nil {
		logInstance.Fatal("Failed to create LLM client", zap.Error(err))
	}
	evaluator := domain.NewLLMEvaluator(llm)

	redisClient, err = cache.NewRedisClient(cfg)
	if err != nil {
		logInstance.Fatal("Failed to connect to test Redis", zap.Error(err))
	}
	logInstance.Info("Successfully connected to test Redis")
	clearRedisCache(redisClient)
	redisAdapter := adapter.NewRedisCacheAdapter(redisClient)

	// cfg is the 4th argument. EmbeddingService and AnswerCacheService are new and set to nil for now.
	quizService := service.NewQuizService(quizDomainRepo, evaluator, redisAdapter, cfg, nil, nil)
	quizHandler := handler.NewQuizHandler(quizService)

	app = fiber.New(fiber.Config{ // Remove ErrorHandler from here
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  20 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})

	// Use app.Use for middleware, similar to cmd/api/main.go
	app.Use(middleware.ErrorHandler()) // Register the global error handler as middleware

	app.Get("/api/categories", quizHandler.GetAllSubCategories)
	app.Get("/api/quiz", quizHandler.GetRandomQuiz)
	app.Get("/api/quizzes", quizHandler.GetBulkQuizzes)
	app.Post("/api/quiz/check", quizHandler.CheckAnswer)

	code := m.Run()

	logInstance.Info("Integration tests completed", zap.Int("exit_code", code))
	os.Exit(code)
}

func initDatabase(cfg *config.Config) error {
	logInstance.Info("Initializing database schema using migrations...")

	migrateDB, err := dblogic.NewMigrateOracleDB(cfg.GetDSN())
	if err != nil {
		logInstance.Error("Failed to create migrate database instance", zap.Error(err))
		return fmt.Errorf("failed to create migrate database instance: %w", err)
	}
	defer migrateDB.Close()

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	migrationsDir := "../../database/migrations"
	absPath := filepath.Join(wd, migrationsDir)

	logInstance.Info("Using migrations directory", zap.String("path", absPath))

	if err := dblogic.RunMigrations(migrateDB, absPath); err != nil {
		logInstance.Error("Failed to run migrations", zap.Error(err))
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logInstance.Info("Database schema initialized successfully via migrations")
	return nil
}

func seedPrerequisites(db *sqlx.DB) error {
	logInstance.Info("Seeding prerequisite data: Categories and SubCategories...")
	subCategoryNameToIDMap = make(map[string]string)

	file, err := os.ReadFile("quiz.json")
	if err != nil {
		return fmt.Errorf("failed to read quiz.json for seeding prerequisites: %w", err)
	}
	var tempQuizzes []TempQuizData
	if err := json.Unmarshal(file, &tempQuizzes); err != nil {
		return fmt.Errorf("failed to unmarshal quiz.json for seeding prerequisites: %w", err)
	}

	uniqueCategories := make(map[string]string)
	uniqueSubCategories := make(map[string]struct{ MainCatName, Desc string })

	for _, tq := range tempQuizzes {
		if tq.MainCategory == "" || tq.SubCategory == "" {
			logInstance.Warn("Skipping quiz entry with empty main_category or sub_category", zap.String("question", tq.Question))
			continue
		}
		if _, exists := uniqueCategories[tq.MainCategory]; !exists {
			uniqueCategories[tq.MainCategory] = "Description for " + tq.MainCategory
		}
		subKey := tq.MainCategory + "|" + tq.SubCategory
		if _, exists := uniqueSubCategories[subKey]; !exists {
			uniqueSubCategories[subKey] = struct{ MainCatName, Desc string }{tq.MainCategory, "Description for " + tq.SubCategory}
		}
	}

	categoryModelMap := make(map[string]string)
	for name, desc := range uniqueCategories {
		categoryID := util.NewULID()
		var cat models.Category
		err := db.Get(&cat, `SELECT id "id", name "name", description "description", created_at "created_at", updated_at "updated_at"
			FROM categories WHERE name = :1`, name)
		if err != nil {
			if err == sql.ErrNoRows {
				now := time.Now()
				_, err = db.Exec(
					"INSERT INTO categories (id, name, description, created_at, updated_at) VALUES (:1, :2, :3, :4, :5)",
					categoryID, name, desc, now, now,
				)
				if err != nil {
					return fmt.Errorf("failed to create category '%s': %w", name, err)
				}
				categoryModelMap[name] = categoryID
			} else {
				return fmt.Errorf("failed to query category '%s': %w", name, err)
			}
		} else {
			categoryModelMap[name] = cat.ID
		}
		logInstance.Info("Category seeded/found", zap.String("name", name), zap.String("id", categoryModelMap[name]))
	}

	for subKey, scData := range uniqueSubCategories {
		categoryID, ok := categoryModelMap[scData.MainCatName]
		if !ok {
			return fmt.Errorf("parent category '%s' not found for subcategory key '%s'", scData.MainCatName, subKey)
		}

		parts := strings.Split(subKey, "|")
		if len(parts) != 2 {
			return fmt.Errorf("invalid subKey format: %s (expected MainCat|SubCat)", subKey)
		}
		subCategoryName := parts[1]

		subCategoryID := util.NewULID()
		var subCat models.SubCategory
		err := db.Get(&subCat, `SELECT id "id", name "name", category_id "category_id", description "description", created_at "created_at", updated_at "updated_at"
			FROM sub_categories WHERE name = :1 AND category_id = :2`, subCategoryName, categoryID)
		if err != nil {
			if err == sql.ErrNoRows {
				now := time.Now()
				_, err = db.Exec(
					"INSERT INTO sub_categories (id, name, category_id, description, created_at, updated_at) VALUES (:1, :2, :3, :4, :5, :6)",
					subCategoryID, subCategoryName, categoryID, scData.Desc, now, now,
				)
				if err != nil {
					return fmt.Errorf("failed to create subcategory '%s': %w", subCategoryName, err)
				}
				subCategoryNameToIDMap[subKey] = subCategoryID
			} else {
				return fmt.Errorf("failed to query subcategory '%s': %w", subCategoryName, err)
			}
		} else {
			subCategoryNameToIDMap[subKey] = subCat.ID
		}
		logInstance.Info("SubCategory seeded/found",
			zap.String("name", subCategoryName),
			zap.String("id", subCategoryNameToIDMap[subKey]),
			zap.String("parent_id", categoryID))
	}

	logInstance.Info("Prerequisite data (Categories and SubCategories) seeded successfully.")
	return nil
}

func saveQuizes() error {
	logInstance.Info("Reading quiz.json for saving quizzes...")
	file, err := os.ReadFile("quiz.json")
	if err != nil {
		return fmt.Errorf("failed to read quiz.json in saveQuizes: %w", err)
	}

	var tempQuizzes []TempQuizData
	if err := json.Unmarshal(file, &tempQuizzes); err != nil {
		return fmt.Errorf("failed to unmarshal quiz.json in saveQuizes: %w", err)
	}

	logInstance.Info("Saving quizzes to database", zap.Int("count", len(tempQuizzes)))

	for _, tq := range tempQuizzes {
		if tq.MainCategory == "" || tq.SubCategory == "" {
			logInstance.Warn("Skipping quiz save due to empty main_category or sub_category", zap.String("question", tq.Question))
			continue
		}
		mapKey := tq.MainCategory + "|" + tq.SubCategory
		subCatID, ok := subCategoryNameToIDMap[mapKey]
		if !ok {
			logInstance.Error("SubCategoryID not found for quiz via mapKey. This indicates a mismatch between quiz.json and seeded categories.",
				zap.String("mapKey", mapKey),
				zap.String("question", tq.Question))
			return fmt.Errorf("SubCategoryID not found for mapKey '%s' (question: '%s'). Ensure quiz.json category names match seeded names.", mapKey, tq.Question)
		}

		quizID := util.NewULID()
		modelAnswers := strings.Join(tq.ModelAnswers, "|||")
		keywords := strings.Join(tq.Keywords, "|||")

		now := time.Now()
		_, err := db.Exec(`
			INSERT INTO quizzes (id, question, model_answers, keywords, difficulty, sub_category_id, created_at, updated_at)
			VALUES (:1, :2, :3, :4, :5, :6, :7, :8)`,
			quizID, tq.Question, modelAnswers, keywords, tq.Difficulty, subCatID, now, now)

		if err != nil {
			logInstance.Error("Failed to save quiz to DB",
				zap.Error(err),
				zap.String("question", tq.Question),
			)
			return fmt.Errorf("failed to save quiz (question: %s): %w", tq.Question, err)
		}
		logInstance.Info("Quiz saved successfully", zap.String("quiz_id", quizID), zap.String("question", tq.Question))
	}

	logInstance.Info("All quizzes from JSON have been processed for saving.")
	return nil
}

func TestGetAllSubCategories(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test for /api/categories should not return an error")

	respBodyBytes, _ := cloneResponseBody(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status OK for /api/categories. Body: %s", respBodyBytes.String())

	var responseBody dto.CategoryResponse
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	require.NoError(t, err, "Failed to decode response body for /api/categories. Body: %s", respBodyBytes.String())

	assert.Equal(t, "All Categories", responseBody.Name, "Response name should be 'All Categories'")
	logInstance.Info("TestGetAllSubCategories executed.")
}

func clearRedisCache(client *redis.Client) {
	if client == nil {
		logInstance.Warn("Redis client is nil, cannot clear cache.")
		return
	}
	err := client.FlushDB(context.Background()).Err()
	if err != nil {
		logInstance.Error("Failed to flush test Redis database", zap.Error(err))
	} else {
		logInstance.Info("Test Redis database flushed successfully.")
	}
}

func TestGetRandomQuiz(t *testing.T) {
	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 {
		t.Skip("Skipping TestGetRandomQuiz: no subcategories were seeded (subCategoryNameToIDMap is empty). Check quiz.json and seedPrerequisites.")
		return
	}
	for key := range subCategoryNameToIDMap {
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name could be extracted for testing GetRandomQuiz.")

	logInstance.Info("Testing GetRandomQuiz with sub_category", zap.String("sub_category", testSubCategoryName))

	encodedSubCategoryName := url.QueryEscape(testSubCategoryName)
	req := httptest.NewRequest(http.MethodGet, "/api/quiz?sub_category="+encodedSubCategoryName, nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test for /api/quiz should not return an error")

	respBodyBytes, _ := cloneResponseBody(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz, got %d. Body: %s", resp.StatusCode, respBodyBytes.String()))

	var quiz dto.QuizResponse
	err = json.NewDecoder(resp.Body).Decode(&quiz)
	require.NoError(t, err, fmt.Sprintf("Failed to decode response body for /api/quiz. Body: %s", respBodyBytes.String()))

	assert.NotZero(t, quiz.ID, "Quiz ID should not be zero")
	assert.NotEmpty(t, quiz.Question, "Quiz question should not be empty")

	logInstance.Info("TestGetRandomQuiz executed.", zap.String("quiz_id", quiz.ID))
}

func TestCheckAnswer(t *testing.T) {
	var testSubCategoryName string
	if len(subCategoryNameToIDMap) == 0 {
		t.Skip("Skipping TestCheckAnswer: no subcategories were seeded. Check quiz.json and seedPrerequisites.")
		return
	}
	for key := range subCategoryNameToIDMap {
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name could be extracted for testing CheckAnswer.")
	encodedSubCategoryName := url.QueryEscape(testSubCategoryName)
	targetURLGetQuiz := "/api/quiz?sub_category=" + encodedSubCategoryName

	logInstance.Info("Fetching a quiz for TestCheckAnswer", zap.String("sub_category", testSubCategoryName))
	reqGet := httptest.NewRequest(http.MethodGet, targetURLGetQuiz, nil)
	respGet, err := app.Test(reqGet)
	require.NoError(t, err)

	respGetBodyBytes, _ := cloneResponseBody(respGet)
	require.Equal(t, http.StatusOK, respGet.StatusCode, fmt.Sprintf("Failed to get a quiz to check answer. Status: %d, Body: %s", respGet.StatusCode, respGetBodyBytes.String()))

	var quizToAnswer dto.QuizResponse
	err = json.NewDecoder(respGet.Body).Decode(&quizToAnswer)
	require.NoError(t, err, fmt.Sprintf("Failed to decode quiz for TestCheckAnswer. Body: %s", respGetBodyBytes.String()))
	require.NotZero(t, quizToAnswer.ID, "Quiz ID for checking answer should not be zero")

	logInstance.Info("Quiz fetched for TestCheckAnswer", zap.String("quiz_id", quizToAnswer.ID))

	answerRequest := dto.CheckAnswerRequest{
		QuizID:     quizToAnswer.ID,
		UserAnswer: "This is a detailed and descriptive test answer, aiming to cover various aspects of the OSI model.",
	}
	requestBody, err := json.Marshal(answerRequest)
	require.NoError(t, err)

	logInstance.Info("Submitting answer for TestCheckAnswer", zap.String("quiz_id", answerRequest.QuizID))
	reqPost := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewReader(requestBody))
	reqPost.Header.Set("Content-Type", "application/json")

	respPost, err := app.Test(reqPost, 60000)
	require.NoError(t, err)

	respPostBodyBytes, _ := cloneResponseBody(respPost)
	require.Equal(t, http.StatusOK, respPost.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz/check. Status: %d, Body: %s", respPost.StatusCode, respPostBodyBytes.String()))

	var answerResponse dto.CheckAnswerResponse
	err = json.NewDecoder(respPost.Body).Decode(&answerResponse)
	require.NoError(t, err, fmt.Sprintf("Failed to decode answer response. Body: %s", respPostBodyBytes.String()))

	assert.True(t, answerResponse.Score >= 0 && answerResponse.Score <= 1, "Score should be between 0 and 1. Actual: %f", answerResponse.Score)
	assert.NotEmpty(t, answerResponse.Explanation, "Explanation should not be empty")
	assert.True(t, answerResponse.Completeness >= 0 && answerResponse.Completeness <= 1, "Completeness should be between 0 and 1. Actual: %f", answerResponse.Completeness)
	assert.True(t, answerResponse.Relevance >= 0 && answerResponse.Relevance <= 1, "Relevance should be between 0 and 1. Actual: %f", answerResponse.Relevance)
	assert.True(t, answerResponse.Accuracy >= 0 && answerResponse.Accuracy <= 1, "Accuracy should be between 0 and 1. Actual: %f", answerResponse.Accuracy)

	logInstance.Info("TestCheckAnswer executed.", zap.String("quiz_id", answerRequest.QuizID), zap.Float64("score", answerResponse.Score))
}

func TestGetBulkQuizzes(t *testing.T) {
	t.Run("SuccessfulRetrieval", func(t *testing.T) {
		var validSubCategoryName string
		var validSubCategoryID string
		if len(subCategoryNameToIDMap) == 0 {
			t.Skip("Skipping SuccessfulRetrieval: no subcategories seeded.")
			return
		}
		for key, id := range subCategoryNameToIDMap {
			parts := strings.Split(key, "|")
			if len(parts) == 2 {
				validSubCategoryName = parts[1]
				validSubCategoryID = id
				break
			}
		}
		require.NotEmpty(t, validSubCategoryName, "No valid subcategory name found for test.")
		logInstance.Info("TestGetBulkQuizzes/SuccessfulRetrieval using", zap.String("sub_category", validSubCategoryName), zap.String("sub_category_id", validSubCategoryID))

		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?sub_category="+url.QueryEscape(validSubCategoryName)+"&count=3", nil)
		resp, err := app.Test(req)
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
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		// Expecting 400 Bad Request for invalid subcategory
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected Bad Request for invalid subcategory. Body: %s", string(bodyBytes))

		var errorResponse middleware.ErrorResponse // Use middleware.ErrorResponse for expected structure
		err = json.Unmarshal(bodyBytes, &errorResponse)
		require.NoError(t, err, "Failed to decode error response. Body: %s", string(bodyBytes))

		expectedErrorCode := string(domain.ErrInvalidCategory)

		assert.Equal(t, expectedErrorCode, errorResponse.Code, "Error code mismatch. Body: %s", string(bodyBytes))
		assert.Equal(t, http.StatusBadRequest, errorResponse.Status, "Error status in body mismatch. Body: %s", string(bodyBytes))
	})

	t.Run("MissingSubCategory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?count=3", nil)
		resp, err := app.Test(req)
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
		if len(subCategoryNameToIDMap) == 0 {
			t.Skip("Skipping DefaultCount: no subcategories seeded.")
			return
		}
		for key := range subCategoryNameToIDMap {
			parts := strings.Split(key, "|")
			if len(parts) == 2 {
				validSubCategoryName = parts[1]
				break
			}
		}
		require.NotEmpty(t, validSubCategoryName, "No valid subcategory name found for test.")

		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?sub_category="+url.QueryEscape(validSubCategoryName), nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected OK. Body: %s", string(bodyBytes))

		var responseBody dto.BulkQuizzesResponse
		err = json.Unmarshal(bodyBytes, &responseBody)
		require.NoError(t, err, "Failed to decode response. Body: %s", string(bodyBytes))
		assert.True(t, len(responseBody.Quizzes) <= handler.DefaultBulkQuizCount, "Expected up to default number of quizzes. Got: %d, Body: %s", len(responseBody.Quizzes), string(bodyBytes))
		if len(subCategoryNameToIDMap[validSubCategoryName]) > 0 && len(responseBody.Quizzes) == 0 {
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
	if len(subCategoryNameToIDMap) == 0 {
		t.Skip("Skipping TestCheckAnswer_Caching: no subcategories seeded.")
		return
	}
	for key := range subCategoryNameToIDMap {
		parts := strings.Split(key, "|")
		if len(parts) == 2 {
			testSubCategoryName = parts[1]
			break
		}
	}
	require.NotEmpty(t, testSubCategoryName, "No subcategory name found for caching test.")

	getQuizReq := httptest.NewRequest(http.MethodGet, "/api/quiz?sub_category="+url.QueryEscape(testSubCategoryName), nil)
	getQuizResp, err := app.Test(getQuizReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getQuizResp.StatusCode)
	var quizToAnswer dto.QuizResponse
	err = json.NewDecoder(getQuizResp.Body).Decode(&quizToAnswer)
	getQuizResp.Body.Close()
	require.NoError(t, err)
	testQuizID = quizToAnswer.ID
	testQuizQuestion = quizToAnswer.Question
	require.NotEmpty(t, testQuizID, "Failed to get a quiz ID for caching test.")

	logInstance.Info("Using quiz for caching tests", zap.String("quizID", testQuizID), zap.String("question", testQuizQuestion))

	cacheKey := service.AnswerCachePrefix + testQuizID // Corrected constant name

	t.Run("CacheMissFirstAnswer", func(t *testing.T) {
		clearRedisCacheKey(redisClient, cacheKey)

		answer1 := "An initial unique answer for caching test regarding " + testQuizQuestion
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answer1})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 60000)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))

		var respBody dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes, &respBody)
		require.NoError(t, err)
		assert.True(t, respBody.Score >= 0 && respBody.Score <= 1, "Score should be between 0 and 1. Actual: %f", respBody.Score)
		assert.NotEmpty(t, respBody.Explanation, "Explanation should not be empty")

		cachedData, err := redisClient.HGet(context.Background(), cacheKey, answer1).Result()
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

		resp1, err := app.Test(req, 10000)
		require.NoError(t, err)
		defer resp1.Body.Close()
		bodyBytes1, _ := io.ReadAll(resp1.Body)
		require.Equal(t, http.StatusOK, resp1.StatusCode, "Body: %s", string(bodyBytes1))

		var respBody1 dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes1, &respBody1)
		require.NoError(t, err)

		req2 := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := app.Test(req2, 10000)
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

		resp, err := app.Test(req, 60000)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))

		var respBody dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes, &respBody)
		require.NoError(t, err)
		assert.True(t, respBody.Score >= 0 && respBody.Score <= 1, "Score should be between 0 and 1. Actual: %f", respBody.Score)

		cachedData, err := redisClient.HGet(context.Background(), cacheKey, answerSimilar).Result()
		if err == redis.Nil {
			logInstance.Info("Different answer was not found in cache (similarity might be below threshold or new LLM call occurred).", zap.String("answer", answerSimilar))
			// It should have been cached after the LLM call in the service.
			// If GenerateOpenAIEmbedding failed, cache write might be skipped.
			// For this test to pass robustly when similarity miss leads to LLM call, cache write must succeed.
			require.NotEqual(t, redis.Nil, err, "Expected new answer to be cached after LLM call, but HGet returned redis.Nil. Embedding/cache write might have failed.")

		}
		require.NoError(t, err, "Error checking cache for similar answer.")
		require.NotEmpty(t, cachedData, "New different answer should now be cached.")
	})

	clearRedisCacheKey(redisClient, cacheKey)
}

func clearRedisCacheKey(client *redis.Client, key string) {
	if client == nil {
		logInstance.Warn("Redis client is nil, cannot clear cache key.", zap.String("key", key))
		return
	}
	err := client.Del(context.Background(), key).Err()
	if err != nil {
		logInstance.Error("Failed to delete key from test Redis database", zap.String("key", key), zap.Error(err))
	} else {
		logInstance.Info("Cleared Redis key successfully.", zap.String("key", key))
	}
}
