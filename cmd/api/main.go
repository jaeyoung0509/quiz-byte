package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
	"quiz-byte/internal/handler"
	"net/http"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/middleware"
	"quiz-byte/internal/repository"
	"quiz-byte/internal/service"
	"strconv"
	"syscall"
	"github.com/tmc/langchaingo/llms/ollama"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"go.uber.org/zap"
)

// requestLogger is a middleware that logs HTTP requests
func requestLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
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

	// Configure HTTP client for Ollama
	ollamaHTTPClient := &http.Client{
		Timeout: 20 * time.Second, // Or from config
	}

	// Create LLM client
	llm, err := ollama.New(
		ollama.WithServerURL(cfg.LLMServer), // Assuming LLMServer is in your config
		ollama.WithModel("qwen3:0.6b"),     // Or from config
		ollama.WithHTTPClient(ollamaHTTPClient),
	)
	if err != nil {
		log.Fatal("Failed to create LLM client", zap.Error(err))
	}

	// Connect to database
	db, err := database.NewOracleDB(cfg.GetDSN())
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize repository
	// The GORM-specific repository instance (repo) is no longer needed here,
	// as NewQuizDatabaseAdapter takes db directly.
	domainRepo := repository.NewQuizDatabaseAdapter(db)

	// Initialize LLM evaluator
	evaluator := domain.NewLLMEvaluator(llm) // Pass the created llm instance

	// Initialize service
	svc := service.NewQuizService(domainRepo, evaluator) // Remove cfg.LLMServer

	// Initialize handler
	handler := handler.NewQuizHandler(svc)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  20 * time.Second, // 읽기 타임아웃
		WriteTimeout: 20 * time.Second, // 쓰기 타임아웃
		IdleTimeout:  20 * time.Second, // 유휴 타임아웃
		BodyLimit:    10 * 1024 * 1024, // 10MB
	})

	// Add request logging middleware
	app.Use(requestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       300, // 5분
	}))
	app.Use(recover.New())
	app.Use(middleware.ErrorHandler())

	// Setup routes
	app.Get("/api/categories", handler.GetAllSubCategories)
	app.Get("/api/quiz", handler.GetRandomQuiz)
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
	}()
}
