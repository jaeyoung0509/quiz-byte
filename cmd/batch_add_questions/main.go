package main

import (
	"context"
	"fmt" // For initial error printing before logger is up
	"time"

	"quiz-byte/internal/adapter"
	"quiz-byte/internal/adapter/embedding"
	"quiz-byte/internal/adapter/quizgen" // Changed from llm to quizgen
	"quiz-byte/internal/cache"
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
	err = logger.Initialize(cfg.Logger) // Corrected initialization
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

	// Initialize Cache Adapter
	var cacheAdapter domain.Cache
	if cfg.Redis.Address != "" {
		redisClient, err := cache.NewRedisClient(cfg.Redis)
		if err != nil {
			logger.Get().Fatal("Failed to initialize Redis Client", zap.Error(err))
		}
		cacheAdapter = adapter.NewRedisCacheAdapter(redisClient)
		logger.Get().Info("Redis Cache initialized successfully.")
	} else {
		logger.Get().Warn("Redis cache is not configured. Running without cache.")
		cacheAdapter = nil
	}

	// Initialize EmbeddingService
	var embedService domain.EmbeddingService
	switch cfg.Embedding.Source {
	case "openai":
		if cfg.Embedding.OpenAI.APIKey == "" {
			logger.Get().Fatal("OpenAI API key is not configured.")
		}
		embeddingCacheTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.Embedding, 24*time.Hour)
		embedService, err = embedding.NewOpenAIEmbeddingService(cfg.Embedding.OpenAI.APIKey, cfg.Embedding.OpenAI.Model, cacheAdapter, embeddingCacheTTL)
		if err != nil {
			logger.Get().Fatal("Failed to initialize OpenAI Embedding Service", zap.Error(err))
		}
		logger.Get().Info("Initialized OpenAI Embedding Service.")
	case "ollama":
		embeddingCacheTTL := cfg.ParseTTLStringOrDefault(cfg.CacheTTLs.Embedding, 24*time.Hour)
		embedService, err = embedding.NewOllamaEmbeddingService(cfg.Embedding.Ollama.ServerURL, cfg.Embedding.Ollama.Model, cacheAdapter, embeddingCacheTTL)
		if err != nil {
			logger.Get().Fatal("Failed to initialize Ollama Embedding Service", zap.Error(err))
		}
		logger.Get().Info("Ollama Embedding Service initialized successfully.")
	default:
		logger.Get().Fatal("Unsupported embedding source specified in configuration", zap.String("source", cfg.Embedding.Source))
	}

	// Initialize LLM Client
	if cfg.LLMProviders.Gemini.APIKey == "" {
		logger.Get().Fatal("Gemini API key is not configured.")
	}
	// Initialize the new QuizGenerator
	quizGenerator, err := quizgen.NewGeminiQuizGenerator(cfg.LLMProviders.Gemini.APIKey, cfg.LLMProviders.Gemini.Model, logger.Get(), cacheAdapter, cfg)
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
