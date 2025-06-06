// @title Quiz Byte API
// @version 1.0
// @description This is the API for the Quiz Byte application.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8090
// @BasePath /api
// @schemes http https
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Type 'Bearer YOUR_JWT_TOKEN' to authorize.
package main

import (
	"context"
	"fmt" // For error formatting
	"log"
	"net/http"
	"os"
	"os/signal"
	"quiz-byte/internal/adapter"
	"quiz-byte/internal/adapter/embedding"
	"quiz-byte/internal/adapter/evaluator" // Added for NewLLMEvaluator
	"quiz-byte/internal/cache"
	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/handler"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/repository"
	"quiz-byte/internal/service"
	"strconv"
	"syscall"
	"time"

	_ "quiz-byte/docs"

	"github.com/gofiber/swagger"
	"github.com/tmc/langchaingo/llms/ollama"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
)

// requestLogger is a middleware that logs HTTP requests
func requestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		path := c.Path()
		method := c.Method()

		// Process request
		err := c.Next()

		// Log request details
		duration := time.Since(start)
		status := c.Response().StatusCode()

		logger.Get().Info("HTTP Request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
		)

		return err
	}
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	if err := logger.Initialize(cfg.Logger); err != nil { // Pass cfg.Logger
		panic(err)
	}
	appLogger := logger.Get() // Renamed log to appLogger for clarity
	defer logger.Sync()

	// Initialize Redis Client
	redisClient, err := cache.NewRedisClient(cfg.Redis) // Pass cfg.Redis
	if err != nil {
		appLogger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	appLogger.Info("Successfully connected to Redis")
	cacheAdapter := adapter.NewRedisCacheAdapter(redisClient)

	// Initialize Embedding Service (remains the same)
	var embeddingService domain.EmbeddingService
	switch cfg.Embedding.Source {
	case "ollama":
		appLogger.Info("Initializing Ollama Embedding Service", zap.String("server_url", cfg.Embedding.Ollama.ServerURL), zap.String("model", cfg.Embedding.Ollama.Model))
		embeddingTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.Embedding, 7*24*time.Hour)
		embeddingService, err = embedding.NewOllamaEmbeddingService(cfg.Embedding.Ollama.ServerURL, cfg.Embedding.Ollama.Model, cacheAdapter, embeddingTTL) // Pass embeddingTTL
		if err != nil {
			appLogger.Fatal("Failed to create Ollama Embedding Service", zap.Error(err))
		}
		appLogger.Info("Ollama Embedding Service initialized successfully")
	case "openai":
		appLogger.Info("Initializing OpenAI Embedding Service", zap.String("model", cfg.Embedding.OpenAI.Model))
		embeddingTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.Embedding, 7*24*time.Hour)
		embeddingService, err = embedding.NewOpenAIEmbeddingService(cfg.Embedding.OpenAI.APIKey, cfg.Embedding.OpenAI.Model, cacheAdapter, embeddingTTL) // Pass embeddingTTL
		if err != nil {
			appLogger.Fatal("Failed to create OpenAI Embedding Service", zap.Error(err))
		}
		appLogger.Info("OpenAI Embedding Service initialized successfully")
	default:
		appLogger.Fatal(fmt.Sprintf("Unsupported embedding source: %s. Please check EMBEDDING_SOURCE in config.", cfg.Embedding.Source))
	}

	// Configure HTTP client for Ollama
	ollamaHTTPClient := &http.Client{Timeout: 20 * time.Second}
	// Use LLMProviders.OllamaServerURL from config
	llm, err := ollama.New(ollama.WithServerURL(cfg.LLMProviders.OllamaServerURL), ollama.WithModel("qwen3:0.6b"), ollama.WithHTTPClient(ollamaHTTPClient))
	if err != nil {
		appLogger.Fatal("Failed to create LLM client", zap.Error(err))
	}

	// Connect to database
	db, err := database.NewSQLXOracleDB(cfg.GetDSN())
	if err != nil {
		appLogger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize repositories
	quizRepository := repository.NewQuizDatabaseAdapter(db) // Renamed for clarity
	userRepository := repository.NewSQLXUserRepository(db)
	userQuizAttemptRepository := repository.NewSQLXUserQuizAttemptRepository(db)

	// Initialize LLM evaluator
	evaluatorService := evaluator.NewLLMEvaluator(llm)

	appLogger.Info("RedisCacheAdapter initialized")

	// Initialize AnswerCacheService with parsed TTL and threshold
	answerEvaluationTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.AnswerEvaluation, 24*time.Hour) // Default from original service
	embeddingSimilarityThreshold := cfg.Embedding.SimilarityThreshold
	answerCacheSvc := service.NewAnswerCacheService(cacheAdapter, quizRepository, answerEvaluationTTL, embeddingSimilarityThreshold)
	appLogger.Info("AnswerCacheService initialized")

	// Initialize QuizService with parsed TTLs
	categoryListTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.CategoryList, 24*time.Hour) // Default from original service
	quizListTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.QuizList, 1*time.Hour)          // Default from original service
	quizService := service.NewQuizService(
		quizRepository,
		evaluatorService,
		cacheAdapter, // This is the domain.Cache for QuizService's own cache needs (e.g. category list)
		embeddingService,
		answerCacheSvc, // Pass the initialized AnswerCacheService
		categoryListTTL,
		quizListTTL,
	)
	appLogger.Info("QuizService initialized")

	authService, err := service.NewAuthService(userRepository, cfg.Auth) // Pass cfg.Auth
	if err != nil {
		appLogger.Fatal("Failed to create AuthService", zap.Error(err))
	}
	appLogger.Info("AuthService initialized")

	userService := service.NewUserService(userRepository, userQuizAttemptRepository, quizRepository) // Remove cfg
	appLogger.Info("UserService initialized")

	// Initialize AnonymousResultCacheService
	anonymousResultCacheTTL := 5 * time.Minute // As specified in the subtask
	anonymousResultCacheSvc := service.NewAnonymousResultCacheService(cacheAdapter, anonymousResultCacheTTL)
	appLogger.Info("AnonymousResultCacheService initialized", zap.Duration("ttl", anonymousResultCacheTTL))

	// Initialize handlers
	quizHandler := handler.NewQuizHandler(quizService, userService, anonymousResultCacheSvc) // Added anonymousResultCacheSvc
	authHandler := handler.NewAuthHandler(authService)                                       // Remove cfg
	userHandler := handler.NewUserHandler(userService)

	// Initialize validation middleware
	validationMiddleware := middleware.NewValidationMiddleware()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,  // Use from config
		WriteTimeout: cfg.Server.WriteTimeout, // Use from config
		IdleTimeout:  20 * time.Second,        // Keep as is or add to config
		BodyLimit:    10 * 1024 * 1024,        // Keep as is or add to config
		ErrorHandler: middleware.ErrorHandler(),
	})

	// Add request logging middleware (remains the same)
	app.Use(requestLogger())
	app.Use(cors.New(cors.Config{AllowOrigins: "*", AllowMethods: "GET,POST,PUT,DELETE,OPTIONS", AllowHeaders: "Origin,Content-Type,Accept,Authorization", MaxAge: 300}))
	app.Use(recover.New())

	// Swagger handler (remains the same)
	app.Get("/swagger/*", swagger.HandlerDefault)

	// API group
	apiGroup := app.Group("/api")

	// Auth routes
	authGroup := apiGroup.Group("/auth")
	authGroup.Get("/google/login", authHandler.GoogleLogin)
	authGroup.Get("/google/callback", authHandler.GoogleCallback)
	authGroup.Post("/refresh", authHandler.RefreshToken)
	authGroup.Post("/logout", middleware.Protected(authService), authHandler.Logout) // Protected logout

	// User routes (all protected)
	userGroup := apiGroup.Group("/users", middleware.Protected(authService))
	userGroup.Get("/me", userHandler.GetMyProfile)
	userGroup.Get("/me/attempts", userHandler.GetMyAttempts)
	userGroup.Get("/me/incorrect-answers", userHandler.GetMyIncorrectAnswers)
	userGroup.Get("/me/recommendations", userHandler.GetMyRecommendations)

	// Quiz and Category routes
	apiGroup.Get("/categories", quizHandler.GetAllSubCategories) // Categories can remain public
	// Apply OptionalAuth to routes that can be accessed by both authenticated and anonymous users
	apiGroup.Get("/quiz", middleware.OptionalAuth(authService), validationMiddleware.ValidateSubCategory(), quizHandler.GetRandomQuiz)
	apiGroup.Get("/quizzes", middleware.OptionalAuth(authService), validationMiddleware.ValidateBulkQuizzesParams(), quizHandler.GetBulkQuizzes)
	apiGroup.Post("/quiz/check", middleware.OptionalAuth(authService), quizHandler.CheckAnswer) // Apply OptionalAuth here

	// Start server (remains the same)
	go func() {
		appLogger.Info("Starting server", zap.Int("port", cfg.Server.Port), zap.String("env", os.Getenv("ENV")))
		if err := app.Listen(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
			appLogger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Graceful shutdown (remains the same)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		appLogger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	appLogger.Info("Server exited gracefully")
}
