package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io" // io.NopCloser 사용을 위해 추가
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository" // 실제 repository 구현체 사용을 위해 import
	"quiz-byte/internal/repository/models"
	"strings" // strings 패키지 import
	"testing"
	"time"

	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/handler"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/service"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var app *fiber.App
var logInstance *zap.Logger // 전역 로거 인스턴스
var db *gorm.DB             // 전역 DB 인스턴스

var subCategoryNameToIDMap map[string]int64

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
	db, err = database.NewOracleDB(dsn)
	if err != nil {
		logInstance.Fatal("Failed to connect to database", zap.Error(err))
	}

	if err := initDatabase(db); err != nil {
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

	evaluator := domain.NewLLMEvaluator(llm)
	quizService := service.NewQuizService(quizDomainRepo, evaluator)
	quizHandler := handler.NewQuizHandler(quizService)

	app = fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  20 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})

	app.Get("/api/categories", quizHandler.GetAllSubCategories)
	app.Get("/api/quiz", quizHandler.GetRandomQuiz)
	app.Post("/api/quiz/check", quizHandler.CheckAnswer)

	code := m.Run()

	logInstance.Info("Integration tests completed", zap.Int("exit_code", code))
	os.Exit(code)
}

func initDatabase(db *gorm.DB) error {
	logInstance.Info("Initializing database schema (unconditional drop, single AutoMigrate call)...")
	tablesToDropUpperCase := []string{
		models.Answer{}.TableName(),
		models.Quiz{}.TableName(),
		models.SubCategory{}.TableName(),
		models.Category{}.TableName(),
	}
	for _, tableName := range tablesToDropUpperCase {
		logInstance.Info("Attempting to drop table (if exists) using raw SQL", zap.String("table", tableName))
		// CASCADE CONSTRAINTS 옵션을 사용하여 관련 제약 조건도 함께 삭제
		// Oracle 23c 이전에는 DROP TABLE IF EXISTS가 없으므로, 에러 핸들링 필요
		dropSQL := fmt.Sprintf("DROP TABLE %s CASCADE CONSTRAINTS", tableName)
		if err := db.Exec(dropSQL).Error; err != nil {
			// ORA-00942: table or view does not exist - 테이블이 없다는 의미이므로 정상 처리
			if strings.Contains(strings.ToUpper(err.Error()), "ORA-00942") {
				logInstance.Info("Table does not exist or already dropped, skipping error.", zap.String("table", tableName), zap.Error(err))
			} else {
				// 다른 SQL 오류는 문제 상황으로 간주
				logInstance.Error("Failed to drop table with raw SQL (critical error)", zap.String("table", tableName), zap.Error(err))
				return fmt.Errorf("failed to drop table %s: %w", tableName, err)
			}
		} else {
			logInstance.Info("Table dropped successfully using raw SQL (or was already gone)", zap.String("table", tableName))
		}
	}

	// Single AutoMigrate call for all models
	logInstance.Info("AutoMigrating all tables in a single call...")
	allModelsToCreate := []interface{}{ // Order for creation (dependencies first)
		&models.Category{},
		&models.SubCategory{},
		&models.Quiz{},
		&models.Answer{},
	}
	if err := db.AutoMigrate(allModelsToCreate...); err != nil { // Pass models as variadic arguments
		logInstance.Error("Failed to AutoMigrate tables", zap.Error(err))
		return fmt.Errorf("failed to AutoMigrate models: %w", err)
	}

	logInstance.Info("Database schema initialized successfully.")
	return nil
}

func seedPrerequisites(db *gorm.DB) error {
	logInstance.Info("Seeding prerequisite data: Categories and SubCategories...")
	subCategoryNameToIDMap = make(map[string]int64)

	file, err := os.ReadFile("quiz.json") // Assuming quiz.json is in the same directory as the test
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
		if tq.MainCategory == "" || tq.SubCategory == "" { // Skip entries with empty category names
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

	categoryModelMap := make(map[string]models.Category)
	for name, desc := range uniqueCategories {
		cat := models.Category{Name: name, Description: desc}
		if errDb := db.FirstOrCreate(&cat, models.Category{Name: name}).Error; errDb != nil {
			return fmt.Errorf("failed to seed category '%s': %w", name, errDb)
		}
		categoryModelMap[name] = cat
		logInstance.Info("Category seeded/found", zap.String("name", cat.Name), zap.Int64("id", cat.ID))
	}

	for subKey, scData := range uniqueSubCategories {
		parentCat, ok := categoryModelMap[scData.MainCatName]
		if !ok {
			return fmt.Errorf("parent category '%s' not found for subcategory key '%s'", scData.MainCatName, subKey)
		}

		parts := strings.Split(subKey, "|")
		if len(parts) != 2 {
			return fmt.Errorf("invalid subKey format: %s (expected MainCat|SubCat)", subKey)
		}
		subCategoryName := parts[1]

		subCat := models.SubCategory{
			Name:        subCategoryName,
			CategoryID:  parentCat.ID,
			Description: scData.Desc,
		}
		if errDb := db.FirstOrCreate(&subCat, models.SubCategory{Name: subCategoryName, CategoryID: parentCat.ID}).Error; errDb != nil {
			return fmt.Errorf("failed to seed subcategory '%s': %w", subCategoryName, errDb)
		}
		subCategoryNameToIDMap[subKey] = subCat.ID
		logInstance.Info("SubCategory seeded/found", zap.String("name", subCat.Name), zap.Int64("id", subCat.ID), zap.Int64("parent_id", subCat.CategoryID))
	}

	logInstance.Info("Prerequisite data (Categories and SubCategories) seeded successfully.")
	return nil
}

func saveQuizes() error {
	logInstance.Info("Reading quiz.json for saving quizzes...")
	file, err := os.ReadFile("quiz.json") // Assuming quiz.json is in the same directory
	if err != nil {
		return fmt.Errorf("failed to read quiz.json in saveQuizes: %w", err)
	}

	var tempQuizzes []TempQuizData
	if err := json.Unmarshal(file, &tempQuizzes); err != nil {
		return fmt.Errorf("failed to unmarshal quiz.json in saveQuizes: %w", err)
	}

	logInstance.Info("Saving quizzes to database", zap.Int("count", len(tempQuizzes)))

	for _, tq := range tempQuizzes {
		if tq.MainCategory == "" || tq.SubCategory == "" { // Skip entries with empty category names, consistent with seedPrerequisites
			logInstance.Warn("Skipping quiz save due to empty main_category or sub_category", zap.String("question", tq.Question))
			continue
		}
		mapKey := tq.MainCategory + "|" + tq.SubCategory
		subCatID, ok := subCategoryNameToIDMap[mapKey]
		if !ok {
			logInstance.Error("SubCategoryID not found for quiz via mapKey. This indicates a mismatch between quiz.json and seeded categories.",
				zap.String("mapKey", mapKey),
				zap.String("question", tq.Question))
			// Potentially return an error or skip this quiz
			return fmt.Errorf("SubCategoryID not found for mapKey '%s' (question: '%s'). Ensure quiz.json category names match seeded names.", mapKey, tq.Question)
		}

		quiz := models.Quiz{
			Question:      tq.Question,
			ModelAnswers:  strings.Join(tq.ModelAnswers, ","),
			Keywords:      strings.Join(tq.Keywords, ","),
			Difficulty:    tq.Difficulty,
			SubCategoryID: subCatID,
		}

		if quiz.ModelAnswers == "" {
			quiz.ModelAnswers = ""
		}
		if quiz.Keywords == "" {
			quiz.Keywords = ""
		}

		if errDb := db.Create(&quiz).Error; errDb != nil {
			logInstance.Error("Failed to save quiz to DB",
				zap.Error(errDb),
				zap.String("question", quiz.Question),
			)
			return fmt.Errorf("failed to save quiz (question: %s): %w", quiz.Question, errDb)
		}
		logInstance.Info("Quiz saved successfully", zap.Int64("quiz_id", quiz.ID), zap.String("question", quiz.Question))
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
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 60 * time.Second})
	require.NoError(t, err, "app.Test for /api/quiz should not return an error")

	respBodyBytes, _ := cloneResponseBody(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz, got %d. Body: %s", resp.StatusCode, respBodyBytes.String()))

	var quiz dto.QuizResponse
	err = json.NewDecoder(resp.Body).Decode(&quiz)
	require.NoError(t, err, fmt.Sprintf("Failed to decode response body for /api/quiz. Body: %s", respBodyBytes.String()))

	assert.NotZero(t, quiz.ID, "Quiz ID should not be zero")
	assert.NotEmpty(t, quiz.Question, "Quiz question should not be empty")

	logInstance.Info("TestGetRandomQuiz executed.", zap.Int64("quiz_id", quiz.ID))
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
	respGet, err := app.Test(reqGet, fiber.TestConfig{Timeout: 180 * time.Second})
	require.NoError(t, err)

	respGetBodyBytes, _ := cloneResponseBody(respGet)
	require.Equal(t, http.StatusOK, respGet.StatusCode, fmt.Sprintf("Failed to get a quiz to check answer. Status: %d, Body: %s", respGet.StatusCode, respGetBodyBytes.String()))

	var quizToAnswer dto.QuizResponse
	err = json.NewDecoder(respGet.Body).Decode(&quizToAnswer)
	require.NoError(t, err, fmt.Sprintf("Failed to decode quiz for TestCheckAnswer. Body: %s", respGetBodyBytes.String()))
	require.NotZero(t, quizToAnswer.ID, "Quiz ID for checking answer should not be zero")

	logInstance.Info("Quiz fetched for TestCheckAnswer", zap.Int64("quiz_id", quizToAnswer.ID))

	answerRequest := dto.AnswerRequest{
		QuizID:     quizToAnswer.ID,
		UserAnswer: "This is a detailed and descriptive test answer, aiming to cover various aspects.",
	}
	requestBody, err := json.Marshal(answerRequest)
	require.NoError(t, err)

	logInstance.Info("Submitting answer for TestCheckAnswer", zap.Int64("quiz_id", answerRequest.QuizID))
	reqPost := httptest.NewRequest(http.MethodPost, "/api/quiz/check", bytes.NewReader(requestBody))
	reqPost.Header.Set("Content-Type", "application/json")

	respPost, err := app.Test(reqPost, fiber.TestConfig{Timeout: 180 * time.Second})
	require.NoError(t, err)

	respPostBodyBytes, _ := cloneResponseBody(respPost)
	require.Equal(t, http.StatusOK, respPost.StatusCode, fmt.Sprintf("Expected status OK for /api/quiz/check. Status: %d, Body: %s", respPost.StatusCode, respPostBodyBytes.String()))

	var answerResponse dto.AnswerResponse
	err = json.NewDecoder(respPost.Body).Decode(&answerResponse)
	require.NoError(t, err, fmt.Sprintf("Failed to decode answer response. Body: %s", respPostBodyBytes.String()))

	assert.True(t, answerResponse.Score >= 0 && answerResponse.Score <= 1, "Score should be between 0 and 1")
	assert.NotEmpty(t, answerResponse.Explanation, "Explanation should not be empty")

	logInstance.Info("TestCheckAnswer executed.", zap.Int64("quiz_id", answerRequest.QuizID), zap.Float64("score", answerResponse.Score))
}
