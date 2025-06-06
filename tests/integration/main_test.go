package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"quiz-byte/internal/adapter"
	"quiz-byte/internal/adapter/embedding"
	"quiz-byte/internal/adapter/evaluator"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository"
	"quiz-byte/internal/repository/models"
	"strings"
	"testing"
	"time"

	"quiz-byte/internal/config"
	dblogic "quiz-byte/internal/database"
	"quiz-byte/internal/handler"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/middleware" // Import middleware
	"quiz-byte/internal/service"
	"quiz-byte/internal/util"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/tmc/langchaingo/llms/ollama"
	"go.uber.org/zap"

	"quiz-byte/internal/cache"

	"github.com/redis/go-redis/v9"
)

var (
	app         *fiber.App
	logInstance *zap.Logger
	db          *sqlx.DB
	redisClient *redis.Client
	cfg         *config.Config // Will be initialized in TestMain

	subCategoryNameToIDMap map[string]string
	cacheKey               string // Used in TestCheckAnswer_Caching, might need review if it should be global
	seededQuizIDWithEval   string // Stores the QuizID that has a QuizEvaluation seeded
)

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

	loadedCfg, err := config.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}
	cfg = loadedCfg // Assign to global cfg

	// Initialize logger
	if err := logger.Initialize(cfg.Logger); err != nil { // Pass cfg.Logger instead of whole cfg
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	logInstance = logger.Get()
	defer func() {
		if logInstance != nil {
			_ = logInstance.Sync()
		}
	}()

	logInstance.Info("Starting integration tests")

	// Initialize DB
	dsn := cfg.GetDSN()
	logInstance.Info("Connecting to database with DSN", zap.String("dsn", dsn))
	db, err = dblogic.NewSQLXOracleDB(dsn)
	if err != nil {
		logInstance.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize Redis
	redisClient, err = cache.NewRedisClient(cfg.Redis) // Pass cfg.Redis
	if err != nil {
		logInstance.Fatal("Failed to connect to test Redis", zap.Error(err))
	}
	logInstance.Info("Successfully connected to test Redis")
	cacheAdapter := adapter.NewRedisCacheAdapter(redisClient) // Keep this local for now, or make global if needed by tests directly

	// Initialize Embedding Service
	var embeddingService domain.EmbeddingService
	embeddingTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.Embedding, 1*time.Hour) // Default TTL example

	switch cfg.Embedding.Source {
	case "openai":
		embeddingService, err = embedding.NewOpenAIEmbeddingService(cfg.Embedding.OpenAI.APIKey, cfg.Embedding.OpenAI.Model, cacheAdapter, embeddingTTL)
		if err != nil {
			logInstance.Fatal("Failed to initialize OpenAI Embedding Service", zap.Error(err))
		}
		logInstance.Info("OpenAI Embedding Service initialized")
	case "ollama":
		embeddingService, err = embedding.NewOllamaEmbeddingService(cfg.Embedding.Ollama.ServerURL, cfg.Embedding.Ollama.Model, cacheAdapter, embeddingTTL)
		if err != nil {
			logInstance.Fatal("Failed to initialize Ollama Embedding Service", zap.Error(err))
		}
		logInstance.Info("Ollama Embedding Service initialized")
	default:
		logInstance.Fatal("Unsupported embedding source", zap.String("source", cfg.Embedding.Source))
	}

	// Initialize LLM client and Evaluator Service
	ollamaHTTPClient := &http.Client{
		Timeout: 20 * time.Second, // Consider making this configurable
	}
	llm, err := ollama.New(
		ollama.WithServerURL(cfg.LLMProviders.OllamaServerURL),
		ollama.WithModel("qwen3:0.6b"), // Using qwen3 0.6b for evaluation
		ollama.WithHTTPClient(ollamaHTTPClient),
	)
	if err != nil {
		logInstance.Fatal("Failed to create LLM client", zap.Error(err))
	}
	evaluatorService := evaluator.NewLLMEvaluator(llm) // Using evaluator.NewLLMEvaluator from the correct package

	// Initialize Repositories
	quizRepository := repository.NewQuizDatabaseAdapter(db)
	userRepository := repository.NewSQLXUserRepository(db)
	userQuizAttemptRepository := repository.NewSQLXUserQuizAttemptRepository(db)

	// Initialize AnswerCacheService
	answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 10*time.Minute) // Example TTL
	answerCacheSvc := service.NewAnswerCacheService(cacheAdapter, quizRepository, answerEvaluationTTL, cfg.Embedding.SimilarityThreshold)

	// Initialize QuizService
	categoryListTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.CategoryList, 5*time.Minute)
	quizListTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.QuizList, 5*time.Minute)
	quizService := service.NewQuizService(quizRepository, evaluatorService, cacheAdapter, embeddingService, answerCacheSvc, categoryListTTL, quizListTTL)

	// Initialize AuthService
	authService, err := service.NewAuthService(userRepository, cfg.Auth)
	if err != nil {
		logInstance.Fatal("Failed to initialize AuthService", zap.Error(err))
	}

	// Initialize UserService - matches cmd/api/main.go (no cfg)
	userService := service.NewUserService(userRepository, userQuizAttemptRepository, quizRepository)

	// Initialize AnonymousResultCacheService
	anonymousResultCacheTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.LLMResponse, 5*time.Minute)
	anonymousResultCacheSvc := service.NewAnonymousResultCacheService(cacheAdapter, anonymousResultCacheTTL)

	// Initialize Handlers
	quizHandler := handler.NewQuizHandler(quizService, userService, anonymousResultCacheSvc)
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)

	// Initialize Validation Middleware
	validationMiddleware := middleware.NewValidationMiddleware()

	// Create Fiber app
	app = fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler(),
	})

	// Register Routes
	// Auth routes
	authRouterGroup := app.Group("/auth")
	authRouterGroup.Get("/google/login", authHandler.GoogleLogin)
	authRouterGroup.Get("/google/callback", authHandler.GoogleCallback)
	authRouterGroup.Post("/refresh", authHandler.RefreshToken)
	authRouterGroup.Post("/logout", middleware.Protected(authService), authHandler.Logout) // Protected

	// User routes
	userRouterGroup := app.Group("/users", middleware.Protected(authService)) // Protected group
	userRouterGroup.Get("/me", userHandler.GetMyProfile)

	// Quiz routes
	apiGroup := app.Group("/api")
	apiGroup.Get("/categories", quizHandler.GetAllSubCategories)                                                                                 // Public
	apiGroup.Get("/quiz", middleware.OptionalAuth(authService), validationMiddleware.ValidateSubCategory(), quizHandler.GetRandomQuiz)           // Optional Auth & Validation
	apiGroup.Get("/quizzes", middleware.OptionalAuth(authService), validationMiddleware.ValidateBulkQuizzesParams(), quizHandler.GetBulkQuizzes) // Optional Auth & Validation
	apiGroup.Post("/quiz/check", middleware.OptionalAuth(authService), quizHandler.CheckAnswer)                                                  // Optional Auth

	// Run migrations, seed data, and execute tests
	if err := initDatabase(cfg); err != nil { // Pass cfg to initDatabase
		logInstance.Fatal("Failed to initialize database", zap.Error(err))
	}

	if err := seedPrerequisites(db); err != nil {
		logInstance.Fatal("Failed to seed prerequisite data", zap.Error(err))
	}

	if err := saveQuizes(); err != nil {
		logInstance.Fatal("Failed to save quizzes", zap.Error(err))
	}

	clearRedisCache(redisClient) // Clear cache before tests

	code := m.Run()

	logInstance.Info("Integration tests completed", zap.Int("exit_code", code))
	os.Exit(code)
}

func initDatabase(cfg *config.Config) error { // cfg parameter added
	logInstance.Info("Initializing database schema using migrations...")

	// Use the new public migration functions from database package
	err := dblogic.InitializeDatabaseForTests(db)
	if err != nil {
		logInstance.Error("Failed to initialize database for tests", zap.Error(err))
		return fmt.Errorf("failed to initialize database for tests: %w", err)
	}

	logInstance.Info("Database schema initialized successfully via migrations")
	return nil
}

func seedPrerequisites(db *sqlx.DB) error {
	logInstance.Info("Seeding prerequisite data: Categories and SubCategories...")
	subCategoryNameToIDMap = make(map[string]string)

	// Assuming quiz.json is in the same directory as this test file (tests/integration)
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
	// Assuming quiz.json is in the same directory as this test file (tests/integration)
	file, err := os.ReadFile("quiz.json")
	if err != nil {
		return fmt.Errorf("failed to read quiz.json in saveQuizes: %w", err)
	}

	var tempQuizzes []TempQuizData
	if err := json.Unmarshal(file, &tempQuizzes); err != nil {
		return fmt.Errorf("failed to unmarshal quiz.json in saveQuizes: %w", err)
	}

	logInstance.Info("Saving quizzes to database", zap.Int("count", len(tempQuizzes)))

	var evaluationSeededForOneQuiz = false // Flag to ensure we only seed evaluation for one quiz

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

		// Seed QuizEvaluation for the first quiz processed
		if !evaluationSeededForOneQuiz {
			seededQuizIDWithEval = quizID // Store the ID for tests to use
			evalID := util.NewULID()
			scoreRanges := models.StringSlice{"0.8-1.0", "0.5-0.79", "0.0-0.49"}
			// Note: Oracle does not have a native JSON type. CLOB is often used.
			// Ensure the JSON string is correctly handled by your DB driver for CLOB.
			// The models.QuizEvaluation might need to define ScoreEvaluations as string or json.RawMessage
			// and handle marshalling/unmarshalling if the driver doesn't do it automatically.
			// For sqlx and Oracle, often you insert JSON as a string into a CLOB.
			scoreEvalsJSON := `[
              {"score_range": "0.8-1.0", "sample_answers": ["This is an excellent and comprehensive answer covering all key aspects."], "explanation": "Excellent work! Your answer is thorough and accurate."},
              {"score_range": "0.5-0.79", "sample_answers": ["This answer covers some main points but could be more detailed."], "explanation": "Good effort. You've grasped the main concepts, but try to elaborate further next time."},
              {"score_range": "0.0-0.49", "sample_answers": ["This answer is off-topic or incorrect."], "explanation": "It seems there's a misunderstanding. Please review the topic and try again."}
            ]`

			// Assuming quiz_evaluations table has columns: id, quiz_id, score_ranges, score_evaluations, created_at, updated_at, minimum_keywords etc.
			// The score_ranges might need to be converted to a format Oracle understands if models.StringSlice isn't directly mappable
			// by the driver (e.g. to a VARRAY or joined string). For simplicity, if it's a CLOB/VARCHAR2 storing comma-separated values,
			// it might work, but a separate table for ranges is more relational. Given models.StringSlice, it likely expects
			// the driver/sqlx to handle the type conversion (e.g. github.com/lib/pq supports this for Postgres string arrays).
			// For Oracle, this might require a custom type or a driver that supports it.
			// Let's assume for now the driver can handle models.StringSlice to a compatible type or it's a string.
			// If models.StringSlice is a custom type that marshals to a string for DB storage, it's fine.
			// The `score_ranges` column in the DB might be a VARCHAR2 or CLOB.
			// For Oracle, StringSlice might not work directly with `db.Exec` like this unless the driver handles it.
			// A common way for collections is to use a specific type or marshal to string/JSON.
			// Let's try passing scoreRanges as is, assuming the driver or sqlx handles it.
			// If it fails, one would typically convert scoreRanges to a concatenated string or similar.
			// However, the schema for quiz_evaluations.score_ranges IS "text" which implies it's a string in Oracle (CLOB/VARCHAR2).
			// So, we should probably join it.

			scoreRangesStr := strings.Join(scoreRanges, ",") // Example: "0.8-1.0,0.5-0.79,0.0-0.49"

			_, evalErr := db.Exec(`
                INSERT INTO quiz_evaluations (id, quiz_id, score_ranges, score_evaluations, created_at, updated_at, minimum_keywords, required_topics, sample_answers, rubric_details)
                VALUES (:1, :2, :3, :4, :5, :6, :7, :8, :9, :10)`,
				evalID, seededQuizIDWithEval, scoreRangesStr, scoreEvalsJSON, time.Now(), time.Now(), 0, "", "", "", // Assuming empty/default for others
			)
			if evalErr != nil {
				logInstance.Error("Failed to save QuizEvaluation", zap.Error(evalErr), zap.String("quiz_id", seededQuizIDWithEval))
				// Decide if this should be a fatal error for tests
				return fmt.Errorf("failed to save QuizEvaluation for quiz_id %s: %w", seededQuizIDWithEval, evalErr)
			}
			logInstance.Info("QuizEvaluation saved successfully", zap.String("quiz_id", seededQuizIDWithEval), zap.String("eval_id", evalID))
			evaluationSeededForOneQuiz = true
		}
	}

	logInstance.Info("All quizzes from JSON have been processed for saving.")
	return nil
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

// Note: UserIDKey constant is not added here as it's not directly used.
// It's typically internal to the middleware. If needed by tests, it can be added or imported.

// Placeholder for imports that might be needed after full service initialization from cmd/api/main.go
// e.g. specific service or repository types if not already covered.
// "quiz-byte/internal/evaluator" // Example, if NewLLMEvaluator is in a separate package and not domain
// "time" // Already imported
// "net/url" // For quiz_test.go, might not be needed here directly.
// "github.com/stretchr/testify/assert" // Already imported
// "github.com/stretchr/testify/require" // Already imported
// "github.com/tmc/langchaingo/llms/ollama" // Already imported
// "go.uber.org/zap" // Already imported
// "quiz-byte/internal/cache" // Already imported
// "github.com/redis/go-redis/v9" // Already imported

// Further changes for full service initialization as per cmd/api/main.go will be done in TestMain.
// This includes embedding service, LLM client, evaluator service, repositories,
// answer cache service, quiz service, auth service, user service, anonymous result cache service,
// handlers, validation middleware, fiber app, and route registration.
// The current TestMain is a direct move but will be updated.
// The cfg initialization is done. Logger, DB, Redis are initialized.
// Ollama LLM client initialization uses cfg.LLMProviders.OllamaServerURL (corrected)
// QuizService and UserService initializations are placeholders and will be updated to match cmd/api/main.go.
// ErrorHandler is registered. Basic routes from original quiz_test.go are kept for now.
// initDatabase, seedPrerequisites, saveQuizes are present.
// cfg parameter added to initDatabase.
// Migration path in initDatabase adjusted.
// quiz.json path in seedPrerequisites and saveQuizes adjusted to be relative to the current file.
// clearRedisCache is called before m.Run().
