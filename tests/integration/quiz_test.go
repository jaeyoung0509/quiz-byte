package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io" // io.NopCloser 사용을 위해 추가
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository" // 실제 repository 구현체 사용을 위해 import
	"quiz-byte/internal/repository/models"
	"strings"
	"testing"
	"time"

	"quiz-byte/internal/config"
	dblogic "quiz-byte/internal/database" // Aliased import for database package
	"quiz-byte/internal/dto"
	"quiz-byte/internal/handler"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/service"
	"quiz-byte/internal/util"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap"

	"quiz-byte/internal/cache" // For Redis client

	"github.com/redis/go-redis/v9" // For Redis client
)

var app *fiber.App
var logInstance *zap.Logger            // 전역 로거 인스턴스
var db *sqlx.DB                        // 전역 DB 인스턴스
var redisClient *redis.Client          // 전역 Redis 클라이언트 인스턴스
var mockEvaluator *MockAnswerEvaluator // Mock for AnswerEvaluator

var subCategoryNameToIDMap map[string]string

// MockAnswerEvaluator is a mock implementation of domain.AnswerEvaluator
type MockAnswerEvaluator struct {
	EvaluateAnswerFunc   func(question, modelAnswer, userAnswer string, keywords []string) (*domain.Answer, error)
	EvaluateAnswerCalled bool
	EvaluateAnswerCount  int
	LastUserAnswer       string
}

func (m *MockAnswerEvaluator) EvaluateAnswer(question, modelAnswer, userAnswer string, keywords []string) (*domain.Answer, error) {
	m.EvaluateAnswerCalled = true
	m.EvaluateAnswerCount++
	m.LastUserAnswer = userAnswer
	if m.EvaluateAnswerFunc != nil {
		return m.EvaluateAnswerFunc(question, modelAnswer, userAnswer, keywords)
	}
	// Default mock behavior
	return &domain.Answer{
		Score:          0.88, // Default mock score
		Explanation:    "Mocked explanation",
		KeywordMatches: []string{"mocked_keyword"},
		Completeness:   0.9,
		Relevance:      0.9,
		Accuracy:       0.85,
	}, nil
}

func (m *MockAnswerEvaluator) Reset() {
	m.EvaluateAnswerCalled = false
	m.EvaluateAnswerCount = 0
	m.LastUserAnswer = ""
}

type TempQuizData struct {
	MainCategory string   `json:"main_category"`
	SubCategory  string   `json:"sub_category"`
	Question     string   `json:"question"`
	ModelAnswers []string `json:"model_answers"`
	Keywords     []string `json:"keywords"`
	Difficulty   int      `json:"difficulty"`
}

// httptest.Response.Body는 한 번만 읽을 수 있으므로, 로깅 후 다시 읽을 수 있도록 재생성하는 헬퍼 함수
func cloneResponseBody(resp *http.Response) (*bytes.Buffer, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()                                    // 이전 Body 닫기
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 새 Body로 교체
	return bytes.NewBuffer(bodyBytes), nil               // 복사된 바이트 버퍼 반환
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
			errSync := logInstance.Sync()
			if errSync != nil {
				fmt.Printf("Error syncing logger: %v\n", errSync)
			}
		}
	}()

	logInstance.Info("Starting integration tests")

	dsn := cfg.GetDSN()
	logInstance.Info("Connecting to database with DSN", zap.String("dsn", dsn))
	db, err = dblogic.NewSQLXOracleDB(dsn) // Use the alias 'dblogic'
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

	// Create LLM client
	// Configure HTTP client for Ollama
	ollamaHTTPClient := &http.Client{
		Timeout: 20 * time.Second, // Or from config
	}

	llm, err := ollama.New(
		ollama.WithServerURL(cfg.LLMServer), // Assuming LLMServer is in your config
		ollama.WithModel("qwen3:0.6b"),      // Or from config
		ollama.WithHTTPClient(ollamaHTTPClient),
	)
	if err != nil {
		logInstance.Fatal("Failed to create LLM client", zap.Error(err))
	}

	evaluator := domain.NewLLMEvaluator(llm) // Real evaluator for non-caching tests if needed

	// Initialize Redis Client for tests
	redisClient, err = cache.NewRedisClient(cfg)
	if err != nil {
		logInstance.Fatal("Failed to connect to test Redis", zap.Error(err))
	}
	logInstance.Info("Successfully connected to test Redis")
	clearRedisCache(redisClient) // Clear cache at the beginning of the test suite

	// Use MOCK evaluator for the service being tested by default for most handlers
	// Specific tests can re-initialize service with real evaluator if needed.
	// For now, TestCheckAnswer will be refactored to TestCheckAnswer_LLM (real) and TestCheckAnswer_Caching (mock)
	quizService := service.NewQuizService(quizDomainRepo, evaluator, redisClient, cfg.OpenAIAPIKey)
	quizHandler := handler.NewQuizHandler(quizService)

	// This is the main app used by tests.
	// If a test needs the real LLM, it might need to setup its own handler or service instance.
	app = fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  20 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})

	app.Get("/api/categories", quizHandler.GetAllSubCategories)
	app.Get("/api/quiz", quizHandler.GetRandomQuiz)
	app.Get("/api/quizzes", quizHandler.GetBulkQuizzes) // Add route for bulk quizzes
	app.Post("/api/quiz/check", quizHandler.CheckAnswer)

	code := m.Run()

	// Optional: Close Redis client if it has a Close method and it's needed.
	// Typically, go-redis manages connections in a pool, so explicit close might not be required for app lifetime.
	// if redisClient != nil {
	//    redisClient.Close()
	// }

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

	// 현재 작업 디렉토리를 기준으로 migrations 디렉토리 경로 설정
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// tests/integration에서 실행될 때 프로젝트 루트로 이동하여 migrations 디렉토리 찾기
	migrationsDir := "../../database/migrations"
	absPath := filepath.Join(wd, migrationsDir)

	logInstance.Info("Using migrations directory", zap.String("path", absPath))

	// 마이그레이션 실행
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

	categoryModelMap := make(map[string]string) // map[category_name]category_id
	for name, desc := range uniqueCategories {
		categoryID := util.NewULID()
		// Check if category exists
		var cat models.Category
		err := db.Get(&cat, `SELECT id "id", name "name", description "description", created_at "created_at", updated_at "updated_at"
			FROM categories WHERE name = :1`, name)
		if err != nil {
			if err == sql.ErrNoRows {
				// Create new category
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
		// Check if subcategory exists
		var subCat models.SubCategory
		err := db.Get(&subCat, `SELECT id "id", name "name", category_id "category_id", description "description", created_at "created_at", updated_at "updated_at"
			FROM sub_categories WHERE name = :1 AND category_id = :2`, subCategoryName, categoryID)
		if err != nil {
			if err == sql.ErrNoRows {
				// Create new subcategory
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
		modelAnswers := strings.Join(tq.ModelAnswers, ",")
		keywords := strings.Join(tq.Keywords, ",")

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

	respBodyBytes, _ := cloneResponseBody(resp) // Clone for logging/debugging if needed

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status OK for /api/categories. Body: %s", respBodyBytes.String())

	var responseBody dto.CategoryResponse
	err = json.NewDecoder(resp.Body).Decode(&responseBody) // Use the (potentially new) resp.Body
	require.NoError(t, err, "Failed to decode response body for /api/categories. Body: %s", respBodyBytes.String())

	assert.Equal(t, "All Categories", responseBody.Name, "Response name should be 'All Categories'")
	logInstance.Info("TestGetAllSubCategories executed.")
}

// Helper function to clear Redis cache (specific for tests)
func clearRedisCache(client *redis.Client) {
	if client == nil {
		logInstance.Warn("Redis client is nil, cannot clear cache.")
		return
	}
	err := client.FlushDB(context.Background()).Err()
	if err != nil {
		logInstance.Error("Failed to flush test Redis database", zap.Error(err))
		// Depending on test strategy, might want to panic or exit
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
		UserAnswer: "This is a detailed and descriptive test answer, aiming to cover various aspects.",
	}
	requestBody, err := json.Marshal(answerRequest)
	require.NoError(t, err)

	logInstance.Info("Submitting answer for TestCheckAnswer", zap.String("quiz_id", answerRequest.QuizID))
	reqPost := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewReader(requestBody))
	reqPost.Header.Set("Content-Type", "application/json")

	respPost, err := app.Test(reqPost, 30000)
	require.NoError(t, err)

	respPostBodyBytes, _ := cloneResponseBody(respPost)
	require.Equal(t, http.StatusOK, respPost.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz/check. Status: %d, Body: %s", respPost.StatusCode, respPostBodyBytes.String()))

	var answerResponse dto.CheckAnswerResponse
	err = json.NewDecoder(respPost.Body).Decode(&answerResponse)
	require.NoError(t, err, fmt.Sprintf("Failed to decode answer response. Body: %s", respPostBodyBytes.String()))

	assert.True(t, answerResponse.Score >= 0 && answerResponse.Score <= 1, "Score should be between 0 and 1")
	assert.NotEmpty(t, answerResponse.Explanation, "Explanation should not be empty")

	logInstance.Info("TestCheckAnswer executed.", zap.String("quiz_id", answerRequest.QuizID), zap.Float64("score", answerResponse.Score))
	// Ensure mock was called if this test is supposed to hit the evaluator
	assert.True(t, mockEvaluator.EvaluateAnswerCalled, "Expected AnswerEvaluator.EvaluateAnswer to be called")
	mockEvaluator.Reset() // Reset for next test
}

func TestGetBulkQuizzes(t *testing.T) {
	// Test Case 1: Successful retrieval
	t.Run("SuccessfulRetrieval", func(t *testing.T) {
		var validSubCategoryName string
		var validSubCategoryID string
		if len(subCategoryNameToIDMap) == 0 {
			t.Skip("Skipping SuccessfulRetrieval: no subcategories seeded.")
			return
		}
		for key, id := range subCategoryNameToIDMap { // Get a valid subcategory
			parts := strings.Split(key, "|")
			if len(parts) == 2 {
				validSubCategoryName = parts[1]
				validSubCategoryID = id // Not directly used in query, but good to have
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
		// Optionally, verify that these quizzes actually belong to the subCategory if DB state is precisely known
	})

	// Test Case 2: Invalid subcategory
	t.Run("InvalidSubCategory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?sub_category=InvalidSubCategoryName&count=3", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		// This depends on how service/repository handles "sub_category name not found"
		// Assuming GetQuizzesByCriteria returns an empty slice and service maps this to an empty response, not an error.
		// If it's an error (e.g. 400/404), adjust assertion.
		// For now, let's assume it returns 200 OK with empty quizzes if subcategory has no quizzes or is invalid (by name).
		// The current repository implementation GetQuizzesByCriteria takes subCategoryID.
		// The service's GetBulkQuizzes takes subCategoryName. The service would need to map name to ID first.
		// Let's assume the service currently doesn't do this name-to-ID mapping and this might fail or return empty.
		// Given current service impl, it passes subCategory (name) to repo, which expects ID. This will likely lead to 0 results.
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected OK for invalid subcategory (empty result). Body: %s", string(bodyBytes))
		var responseBody dto.BulkQuizzesResponse
		err = json.Unmarshal(bodyBytes, &responseBody)
		require.NoError(t, err, "Failed to decode response. Body: %s", string(bodyBytes))
		assert.Empty(t, responseBody.Quizzes, "Expected empty quizzes for invalid subcategory. Body: %s", string(bodyBytes))
	})

	// Test Case 3: Missing subcategory
	t.Run("MissingSubCategory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/quizzes?count=3", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected Bad Request. Body: %s", string(bodyBytes))
		// Further check error message if desired
	})

	// Test Case 4: Default count
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
		assert.Len(t, responseBody.Quizzes, handler.DefaultBulkQuizCount, "Expected default number of quizzes. Body: %s", string(bodyBytes))
	})
}

func TestCheckAnswer_Caching(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" || os.Getenv("OPENAI_API_KEY") == "YOUR_OPENAI_API_KEY_GOES_HERE" {
		t.Skip("Skipping TestCheckAnswer_Caching: OPENAI_API_KEY is not set or is a placeholder.")
		return
	}

	var testQuizID string
	var testQuizQuestion string // For logging clarity

	// Find a quiz to use for testing
	// Fetch a quiz from a known subcategory to ensure it exists
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

	cacheKey := service.QuizAnswerCachePrefix + testQuizID // Using exported prefix if available, otherwise "quizanswers:"

	// Define a common answer structure for the mock
	mockLLMResponse := &domain.Answer{
		Score:          0.75,
		Explanation:    "This is a explanation from the mock LLM.",
		KeywordMatches: []string{"mock", "llm"},
		Completeness:   0.8,
		Relevance:      0.8,
		Accuracy:       0.7,
	}
	mockEvaluator.EvaluateAnswerFunc = func(question, modelAnswer, userAnswer string, keywords []string) (*domain.Answer, error) {
		return mockLLMResponse, nil
	}

	// --- Test Case 1: Cache Miss (First Answer) ---
	t.Run("CacheMissFirstAnswer", func(t *testing.T) {
		clearRedisCacheKey(redisClient, cacheKey) // Clear specific key before this test
		mockEvaluator.Reset()

		answer1 := "An initial unique answer for caching test."
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answer1})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 30000) // Increased timeout for potential first LLM call (even if mocked)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))
		assert.True(t, mockEvaluator.EvaluateAnswerCalled, "Expected LLM evaluator to be called on cache miss.")
		assert.Equal(t, 1, mockEvaluator.EvaluateAnswerCount, "LLM evaluator should be called once.")

		var respBody dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes, &respBody)
		require.NoError(t, err)
		assert.Equal(t, mockLLMResponse.Score, respBody.Score) // Check if response is from mock
		assert.Equal(t, mockLLMResponse.Explanation, respBody.Explanation)

		// Verify cache entry (optional, but good for sanity)
		cachedData, err := redisClient.HGet(context.Background(), cacheKey, answer1).Result()
		require.NoError(t, err, "Expected answer to be cached in Redis.")
		require.NotEmpty(t, cachedData, "Cached data should not be empty.")
	})

	// --- Test Case 2: Cache Hit (Identical Answer) ---
	t.Run("CacheHitIdenticalAnswer", func(t *testing.T) {
		// This test depends on CacheMissFirstAnswer having run and populated the cache.
		mockEvaluator.Reset()

		answer1 := "An initial unique answer for caching test." // Same answer as above
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answer1})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 10000) // Shorter timeout, should be fast from cache
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))
		assert.False(t, mockEvaluator.EvaluateAnswerCalled, "Expected LLM evaluator NOT to be called on cache hit.")
		assert.Equal(t, 0, mockEvaluator.EvaluateAnswerCount, "LLM evaluator should be called zero times.")

		var respBody dto.CheckAnswerResponse
		err = json.Unmarshal(bodyBytes, &respBody)
		require.NoError(t, err)
		assert.Equal(t, mockLLMResponse.Score, respBody.Score) // Check if response is from (mocked) cache
		assert.Equal(t, mockLLMResponse.Explanation, respBody.Explanation)
	})

	// --- Test Case 3: Cache Miss (Different Answer, Similarity Below Threshold) ---
	t.Run("CacheMissDifferentAnswer", func(t *testing.T) {
		// This also depends on CacheMissFirstAnswer. Cosine similarity with current stubs will be 0.
		mockEvaluator.Reset()

		answer2 := "A completely different answer to ensure cache miss by similarity."
		reqBody, _ := json.Marshal(dto.CheckAnswerRequest{QuizID: testQuizID, UserAnswer: answer2})
		req := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Body: %s", string(bodyBytes))
		assert.True(t, mockEvaluator.EvaluateAnswerCalled, "Expected LLM evaluator to be called for a different answer.")
		assert.Equal(t, 1, mockEvaluator.EvaluateAnswerCount, "LLM evaluator should be called once.")

		// Verify this new answer is also cached (optional)
		cachedData, err := redisClient.HGet(context.Background(), cacheKey, answer2).Result()
		require.NoError(t, err, "Expected new answer to be cached.")
		require.NotEmpty(t, cachedData, "Cached data for new answer should not be empty.")
	})

	// Cleanup after this specific group of tests
	clearRedisCacheKey(redisClient, cacheKey)
	mockEvaluator.Reset() // Reset mock after all sub-tests in this group are done.
}

// Helper function to clear a specific key from Redis
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
