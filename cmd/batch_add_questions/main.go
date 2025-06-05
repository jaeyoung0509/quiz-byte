package main

import (
	"context"
	"fmt" // For initial error printing before logger is up

	"quiz-byte/internal/adapter/embedding"
	"quiz-byte/internal/adapter/quizgen" // Changed from llm to quizgen
	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/repository"
	"quiz-byte/internal/service"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		// Logger might not be initialized yet, so use fmt for this critical error
		fmt.Printf("Failed to load configuration: %v\n", err)
		return // Cannot proceed without config
	}

	// Initialize logger
	err = logger.Initialize(cfg) // Corrected initialization
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return // Cannot proceed without logger
	}
	defer logger.Sync() // Corrected defer

	logger.Get().Info("Batch process starting up...") // Corrected usage

	// Create Oracle DSN
	dsn := cfg.GetDSN()
	if dsn == "" {
		logger.Get().Fatal("Generated DSN is empty. Check DB configuration.")
	}
	// Not logging DSN itself for security
	logger.Get().Info("Successfully generated DSN.")


	// Establish DB connection
	db, err := database.NewSQLXOracleDB(dsn)
	if err != nil {
		logger.Get().Fatal("Failed to connect to Oracle database", zap.Error(err))
	}
	defer db.Close()
	logger.Get().Info("Successfully connected to Oracle database.")

	// Initialize Repositories
	quizRepo := repository.NewQuizDatabaseAdapter(db)
	categoryRepo := repository.NewCategoryDatabaseAdapter(db) // Assuming this constructor exists
	logger.Get().Info("Initialized repositories.")

	// Initialize EmbeddingService
	var embedService domain.EmbeddingService
	switch cfg.Embedding.Source {
	case "openai":
		if cfg.Embedding.OpenAI.APIKey == "" {
			logger.Get().Fatal("OpenAI API key is not configured.")
		}
		embedService, err = embedding.NewOpenAIEmbeddingService(cfg.Embedding.OpenAI.APIKey, cfg.Embedding.OpenAI.Model)
		if err != nil {
			logger.Get().Fatal("Failed to initialize OpenAI Embedding Service", zap.Error(err))
		}
		logger.Get().Info("Initialized OpenAI Embedding Service.")
	// Add other cases for "ollama" or other sources if needed
	default:
		logger.Get().Fatal("Unsupported embedding source specified in configuration", zap.String("source", cfg.Embedding.Source))
	}

	// Initialize LLM Client
	if cfg.Gemini.APIKey == "" {
		logger.Get().Fatal("Gemini API key is not configured.")
	}
	// Initialize the new QuizGenerator
	quizGenerator, err := quizgen.NewGeminiQuizGenerator(cfg.Gemini.APIKey, cfg.Gemini.Model, logger.Get())
	if err != nil {
		logger.Get().Fatal("Failed to initialize QuizGenerator", zap.Error(err))
	}
	logger.Get().Info("Initialized QuizGenerator (Gemini).")

	// Initialize BatchService
	batchSvc := service.NewBatchService(quizRepo, categoryRepo, embedService, quizGenerator, cfg, logger.Get())
	logger.Get().Info("Initialized Batch Service.")

	// Create a context for the batch process
	ctx := context.Background()

	// Call GenerateNewQuizzesAndSave
	logger.Get().Info("Starting quiz generation and saving process...")
	err = batchSvc.GenerateNewQuizzesAndSave(ctx)
	if err != nil {
		logger.Get().Fatal("Batch process failed", zap.Error(err))
	}

	logger.Get().Info("Batch process completed successfully.")
}
