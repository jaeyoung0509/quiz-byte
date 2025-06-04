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
package main

import (
	"context"
	"log"
	"net/http"
	"fmt" // For error formatting
	"os"
	"os/signal"
	"quiz-byte/internal/adapter" // Make sure this import is added
	"quiz-byte/internal/adapter/embedding" // Added
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

	_ "quiz-byte/cmd/api/docs"

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
	if err := logger.Initialize(cfg); err != nil {
		panic(err)
	}
	defer logger.Sync()

	log := logger.Get()

	// Initialize Embedding Service
	var embeddingService domain.EmbeddingService
	switch cfg.Embedding.Source {
	case "ollama":
		log.Info("Initializing Ollama Embedding Service",
			zap.String("server_url", cfg.Embedding.OllamaServerURL),
			zap.String("model", cfg.Embedding.OllamaModel))
		var ollamaErr error
		embeddingService, ollamaErr = embedding.NewOllamaEmbeddingService(cfg.Embedding.OllamaServerURL, cfg.Embedding.OllamaModel)
		if ollamaErr != nil {
			log.Fatal("Failed to create Ollama Embedding Service", zap.Error(ollamaErr))
		}
		log.Info("Ollama Embedding Service initialized successfully")
	case "openai":
		log.Info("Initializing OpenAI Embedding Service",
			zap.String("model", cfg.Embedding.OpenAIModel)) // Assuming model name is relevant here
		var openaiErr error
		// Ensure cfg.OpenAIAPIKey and cfg.Embedding.OpenAIModel are the correct fields from your config struct
		embeddingService, openaiErr = embedding.NewOpenAIEmbeddingService(cfg.OpenAIAPIKey, cfg.Embedding.OpenAIModel)
		if openaiErr != nil {
			log.Fatal("Failed to create OpenAI Embedding Service", zap.Error(openaiErr))
		}
		log.Info("OpenAI Embedding Service initialized successfully")
	default:
		log.Fatal(fmt.Sprintf("Unsupported embedding source: %s. Please check EMBEDDING_SOURCE in config.", cfg.Embedding.Source))
	}

	// Configure HTTP client for Ollama
	ollamaHTTPClient := &http.Client{
		Timeout: 20 * time.Second, // Or from config
	}

	// Create LLM client
	llm, err := ollama.New(
		ollama.WithServerURL(cfg.LLMServer), // Assuming LLMServer is in your config
		ollama.WithModel("qwen3:0.6b"),      // Or from config
		ollama.WithHTTPClient(ollamaHTTPClient),
	)
	if err != nil {
		log.Fatal("Failed to create LLM client", zap.Error(err))
	}

	// Connect to database
	db, err := database.NewSQLXOracleDB(cfg.GetDSN())
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize repository
	// The GORM-specific repository instance (repo) is no longer needed here,
	// as NewQuizDatabaseAdapter takes db directly.
	domainRepo := repository.NewQuizDatabaseAdapter(db)

	// Initialize LLM evaluator
	evaluator := domain.NewLLMEvaluator(llm) // Pass the created llm instance

	// Initialize Redis Client
	redisClient, err := cache.NewRedisClient(cfg)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	log.Info("Successfully connected to Redis")

	// Initialize Cache Adapter (NEW LINES TO ADD)
	cacheAdapter := adapter.NewRedisCacheAdapter(redisClient)
	log.Info("RedisCacheAdapter initialized", zap.String("adapter_type", "RedisCacheAdapter")) // Optional: for confirmation

	// Initialize service
	svc := service.NewQuizService(domainRepo, evaluator, cacheAdapter, cfg, embeddingService)

	// Initialize handler
	handler := handler.NewQuizHandler(svc)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  20 * time.Second, // Read timeout
		WriteTimeout: 20 * time.Second, // Write timeout
		IdleTimeout:  20 * time.Second, // Idle timeout
		BodyLimit:    10 * 1024 * 1024, // 10MB
	})

	// Add request logging middleware
	app.Use(requestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
		MaxAge:       300,
	}))
	app.Use(recover.New())
	app.Use(middleware.ErrorHandler())

	// Swagger handler
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Setup routes
	app.Get("/api/categories", handler.GetAllSubCategories)
	app.Get("/api/quiz", handler.GetRandomQuiz)     // Single random quiz
	app.Get("/api/quizzes", handler.GetBulkQuizzes) // Multiple quizzes by criteria
	app.Post("/api/quiz/check", handler.CheckAnswer)

	// Start server in a goroutine
	go func() {
		log.Info("Starting server",
			zap.Int("port", cfg.Server.Port),
			zap.String("env", os.Getenv("ENV")),
		)

		if err := app.Listen(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited gracefully")

}
