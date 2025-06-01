package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io" // io.NopCloser 사용을 위해 추가
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"              // Added/Ensured
	"path/filepath"   // Added/Ensured
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository" // 실제 repository 구현체 사용을 위해 import
	"quiz-byte/internal/repository/models"
	"sort"            // Added/Ensured
	"strings"         // Ensured
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
)

var app *fiber.App
var logInstance *zap.Logger // 전역 로거 인스턴스
var db *sqlx.DB             // 전역 DB 인스턴스

var subCategoryNameToIDMap map[string]string

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

func initDatabase(db *sqlx.DB) error {
	logInstance.Info("Initializing database schema using migrations...")

	migrationsDir := "../database/migrations"

	wd, err := os.Getwd()
	if err != nil {
		logInstance.Warn("Failed to get current working directory", zap.Error(err))
	} else {
		logInstance.Info("Current working directory for test", zap.String("wd", wd))
		expectedDownMigrationsPath := filepath.Join(wd, migrationsDir)
		logInstance.Info("Expecting to find DOWN migrations in", zap.String("path", expectedDownMigrationsPath))
		if _, statErr := os.Stat(expectedDownMigrationsPath); os.IsNotExist(statErr) {
			logInstance.Error("Down migrations directory does not exist at expected path", zap.String("path", expectedDownMigrationsPath))
		}
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("could not read migrations directory %s: %w. Check CWD and relative path. CWD: %s", migrationsDir, err, wd)
	}

	var downFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".down.sql") {
			downFiles = append(downFiles, file.Name())
		}
	}
	// Sort in reverse filename order
	sort.Sort(sort.Reverse(sort.StringSlice(downFiles)))

	for _, fileName := range downFiles {
		logInstance.Info("Executing down migration", zap.String("file", fileName))
		filePath := filepath.Join(migrationsDir, fileName)
		content, errReadFile := os.ReadFile(filePath)
		if errReadFile != nil {
			return fmt.Errorf("could not read migration file %s: %w", filePath, errReadFile)
		}

		if strings.TrimSpace(string(content)) == "" {
			logInstance.Info("Skipping empty down migration file", zap.String("file", fileName))
			continue
		}

		_, errExec := db.Exec(string(content))
		if errExec != nil {
			type oracleError interface {
				Code() int
			}
			if oErr, ok := errExec.(oracleError); ok {
				if oErr.Code() == 942 || oErr.Code() == 2443 || oErr.Code() == 1418 || oErr.Code() == 2289 {
					logInstance.Warn("Ignoring Oracle error for down migration (object likely did not exist)",
						zap.String("file", fileName), zap.Int("oracle_code", oErr.Code()), zap.Error(errExec))
				} else {
					return fmt.Errorf("oracle error executing down migration %s (code %d): %w", fileName, oErr.Code(), errExec)
				}
			} else {
				if strings.Contains(strings.ToLower(errExec.Error()), "does not exist") ||
					strings.Contains(strings.ToLower(errExec.Error()), "unknown table") ||
					strings.Contains(strings.ToLower(errExec.Error()), "nonexistent constraint") { // For other DBs
					logInstance.Warn("Ignoring generic 'does not exist' error for down migration", zap.String("file", fileName), zap.Error(errExec))
				} else {
					return fmt.Errorf("non-oracle error executing down migration %s: %w", fileName, errExec)
				}
			}
		}
	}

	logInstance.Info("Attempting to run UP migrations using dblogic.RunMigrations.", zap.String("expected_up_migrations_path_for_RunMigrations", filepath.Join(wd, "database/migrations")))

	upMigrationsPathInternal := "database/migrations"
	if _, statErr := os.Stat(filepath.Join(wd, upMigrationsPathInternal)); os.IsNotExist(statErr) {
		logInstance.Error("UP migrations directory (database/migrations) does not exist relative to CWD. dblogic.RunMigrations will likely fail.",
			zap.String("cwd", wd),
			zap.String("path_used_by_RunMigrations", upMigrationsPathInternal),
			zap.String("resolved_path_attempted", filepath.Join(wd, upMigrationsPathInternal)))
	}

	if err := dblogic.RunMigrations(db.DB); err != nil {
		logInstance.Error("Failed to run up migrations via dblogic.RunMigrations", zap.Error(err), zap.String("cwd_when_called", wd))
		return fmt.Errorf("failed to run up migrations (CWD: %s): %w", wd, err)
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
}
