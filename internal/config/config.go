package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// GoogleOAuthConfig holds configuration for Google OAuth.
type GoogleOAuthConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

// JWTConfig holds configuration for JWT.
type JWTConfig struct {
	SecretKey       string        `yaml:"secret_key"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

type Config struct {
	DB          DBConfig
	Server      ServerConfig
	LLMServer   string
	Redis       RedisConfig
	Embedding   EmbeddingConfig
	Gemini      GeminiConfig
	Batch       BatchConfig // New field for Batch operations
	GoogleOAuth GoogleOAuthConfig
	JWT         JWTConfig
	CacheTTLs   CacheTTLConfig // Added CacheTTLs
}

// CacheTTLConfig holds configuration for cache TTLs.
// TTLs are strings to allow parsing from environment variables (e.g., "1h30m").
type CacheTTLConfig struct {
	LLMResponse      string `yaml:"llm_response" env:"CACHE_TTL_LLM_RESPONSE" envDefault:"24h"`
	Embedding        string `yaml:"embedding" env:"CACHE_TTL_EMBEDDING" envDefault:"168h"` // 7 days
	QuizList         string `yaml:"quiz_list" env:"CACHE_TTL_QUIZ_LIST" envDefault:"1h"`
	CategoryList     string `yaml:"category_list" env:"CACHE_TTL_CATEGORY_LIST" envDefault:"24h"`
	AnswerEvaluation string `yaml:"answer_evaluation" env:"CACHE_TTL_ANSWER_EVALUATION" envDefault:"24h"`
	QuizDetail       string `yaml:"quiz_detail" env:"CACHE_TTL_QUIZ_DETAIL" envDefault:"6h"` // For potential future use
}

// BatchConfig holds configuration for batch processes.
type BatchConfig struct {
	NumQuestionsPerSubCategory int `yaml:"num_questions_per_subcategory"`
}

// GeminiConfig holds configuration for the Gemini LLM.
type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type EmbeddingConfig struct {
	Source              string  `yaml:"source"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	Ollama              OllamaEmbeddingConfig `yaml:"ollama"`
	OpenAI              OpenAIEmbeddingConfig `yaml:"openai"`
}

type OllamaEmbeddingConfig struct {
	Model     string `yaml:"model"`
	ServerURL string `yaml:"server_url"`
}

type OpenAIEmbeddingConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add config paths based on environment
	if os.Getenv("ENV") == "test" {
		// For test environment, look for config in the project root
		viper.AddConfigPath("../../config")
		viper.AddConfigPath("../../")
	} else {
		// For production/development environment
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	viper.SetEnvPrefix("APP") // All env vars will need to be prefixed with APP_
	viper.AutomaticEnv()

	// Database environment variables
	viper.BindEnv("db.host", "APP_DB_HOST")
	viper.BindEnv("db.port", "APP_DB_PORT")
	viper.BindEnv("db.user", "APP_DB_USER")
	viper.BindEnv("db.password", "APP_DB_PASSWORD")
	viper.BindEnv("db.name", "APP_DB_NAME")

	// Server environment variables
	viper.BindEnv("server.port", "APP_SERVER_PORT")

	// LLM Server environment variables
	viper.BindEnv("llm.server", "APP_LLM_SERVER")

	// Redis environment variables
	viper.BindEnv("redis.address", "APP_REDIS_ADDRESS")
	viper.BindEnv("redis.password", "APP_REDIS_PASSWORD")
	viper.BindEnv("redis.db", "APP_REDIS_DB")

	// Embedding environment variables
	viper.BindEnv("embedding.source", "APP_EMBEDDING_SOURCE")
	viper.BindEnv("embedding.similarity_threshold", "APP_EMBEDDING_SIMILARITY_THRESHOLD")
	viper.BindEnv("embedding.ollama.model", "APP_EMBEDDING_OLLAMA_MODEL")
	viper.BindEnv("embedding.ollama.server_url", "APP_EMBEDDING_OLLAMA_SERVER_URL")
	viper.BindEnv("embedding.openai.api_key", "APP_EMBEDDING_OPENAI_API_KEY")
	viper.BindEnv("embedding.openai.model", "APP_EMBEDDING_OPENAI_MODEL")

	// Gemini LLM environment variables
	viper.BindEnv("gemini.api_key", "APP_GEMINI_API_KEY")
	viper.BindEnv("gemini.model", "APP_GEMINI_MODEL")

	// Batch process environment variables
	viper.BindEnv("batch.num_questions_per_subcategory", "APP_BATCH_NUM_QUESTIONS_PER_SUBCATEGORY")

	// Google OAuth environment variables
	viper.BindEnv("googleoauth.client_id", "APP_GOOGLE_CLIENT_ID")
	viper.BindEnv("googleoauth.client_secret", "APP_GOOGLE_CLIENT_SECRET")
	viper.BindEnv("googleoauth.redirect_url", "APP_GOOGLE_REDIRECT_URL")

	// JWT environment variables
	viper.BindEnv("jwt.secret_key", "APP_JWT_SECRET_KEY")
	viper.BindEnv("jwt.access_token_ttl", "APP_JWT_ACCESS_TOKEN_TTL")   // Expecting value in seconds
	viper.BindEnv("jwt.refresh_token_ttl", "APP_JWT_REFRESH_TOKEN_TTL") // Expecting value in seconds

	// Cache TTLs environment variables
	viper.BindEnv("cachettls.llm_response", "APP_CACHE_TTL_LLM_RESPONSE")
	viper.BindEnv("cachettls.embedding", "APP_CACHE_TTL_EMBEDDING")
	viper.BindEnv("cachettls.quiz_list", "APP_CACHE_TTL_QUIZ_LIST")
	viper.BindEnv("cachettls.category_list", "APP_CACHE_TTL_CATEGORY_LIST")
	viper.BindEnv("cachettls.answer_evaluation", "APP_CACHE_TTL_ANSWER_EVALUATION")
	viper.BindEnv("cachettls.quiz_detail", "APP_CACHE_TTL_QUIZ_DETAIL")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Log the config file being used
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		absPath, _ := filepath.Abs(configFile)
		fmt.Printf("Using config file: %s\n", absPath)
	}

	config := &Config{
		DB: DBConfig{
			Host:     viper.GetString("db.host"),
			Port:     viper.GetInt("db.port"),
			User:     viper.GetString("db.user"),
			Password: viper.GetString("db.password"),
			DBName:   viper.GetString("db.name"),
		},
		Server: ServerConfig{
			Port:         viper.GetInt("server.port"),
			ReadTimeout:  viper.GetDuration("server.read_timeout") * time.Second,
			WriteTimeout: viper.GetDuration("server.write_timeout") * time.Second,
		},
		LLMServer: viper.GetString("llm.server"),
		Redis: RedisConfig{
			Address:  viper.GetString("redis.address"),
			Password: viper.GetString("redis.password"),
			DB:       viper.GetInt("redis.db"),
		},
		Embedding: EmbeddingConfig{
			Source:              viper.GetString("embedding.source"),
			SimilarityThreshold: viper.GetFloat64("embedding.similarity_threshold"),
			Ollama: OllamaEmbeddingConfig{
				Model:     viper.GetString("embedding.ollama.model"),
				ServerURL: viper.GetString("embedding.ollama.server_url"),
			},
			OpenAI: OpenAIEmbeddingConfig{
				APIKey: viper.GetString("embedding.openai.api_key"),
				Model:  viper.GetString("embedding.openai.model"),
			},
		},
		Gemini: GeminiConfig{
			APIKey: viper.GetString("gemini.api_key"),
			Model:  viper.GetString("gemini.model"),
		},
		Batch: BatchConfig{
			NumQuestionsPerSubCategory: viper.GetInt("batch.num_questions_per_subcategory"),
		},
		GoogleOAuth: GoogleOAuthConfig{
			ClientID:     viper.GetString("googleoauth.client_id"),
			ClientSecret: viper.GetString("googleoauth.client_secret"),
			RedirectURL:  viper.GetString("googleoauth.redirect_url"),
		},
		JWT: JWTConfig{
			SecretKey:       viper.GetString("jwt.secret_key"),
			AccessTokenTTL:  viper.GetDuration("jwt.access_token_ttl") * time.Second,
			RefreshTokenTTL: viper.GetDuration("jwt.refresh_token_ttl") * time.Second,
		},
		CacheTTLs: CacheTTLConfig{
			LLMResponse:      viper.GetString("cachettls.llm_response"),
			Embedding:        viper.GetString("cachettls.embedding"),
			QuizList:         viper.GetString("cachettls.quiz_list"),
			CategoryList:     viper.GetString("cachettls.category_list"),
			AnswerEvaluation: viper.GetString("cachettls.answer_evaluation"),
			QuizDetail:       viper.GetString("cachettls.quiz_detail"),
		},
	}

	// Set default for SimilarityThreshold if not provided or zero
	if !viper.IsSet("embedding.similarity_threshold") || config.Embedding.SimilarityThreshold == 0 {
		config.Embedding.SimilarityThreshold = 0.95 // Default value
	}

	// Set default for Gemini model if not provided
	if config.Gemini.Model == "" {
		config.Gemini.Model = "gemini-pro" // Default model
	}

	// Set default for NumQuestionsPerSubCategory if not provided or zero
	if config.Batch.NumQuestionsPerSubCategory == 0 {
		config.Batch.NumQuestionsPerSubCategory = 3 // Default value
	}

	// Set default for JWT AccessTokenTTL if not provided or zero
	if config.JWT.AccessTokenTTL == 0 {
		config.JWT.AccessTokenTTL = 15 * time.Minute // Default to 15 minutes
	}

	// Set default for JWT RefreshTokenTTL if not provided or zero
	if config.JWT.RefreshTokenTTL == 0 {
		config.JWT.RefreshTokenTTL = 7 * 24 * time.Hour // Default to 7 days
	}

	// Set defaults for CacheTTLs if not provided or empty strings
	if config.CacheTTLs.LLMResponse == "" {
		config.CacheTTLs.LLMResponse = "24h"
	}
	if config.CacheTTLs.Embedding == "" {
		config.CacheTTLs.Embedding = "168h"
	}
	if config.CacheTTLs.QuizList == "" {
		config.CacheTTLs.QuizList = "1h"
	}
	if config.CacheTTLs.CategoryList == "" {
		config.CacheTTLs.CategoryList = "24h"
	}
	if config.CacheTTLs.AnswerEvaluation == "" {
		config.CacheTTLs.AnswerEvaluation = "24h"
	}
	if config.CacheTTLs.QuizDetail == "" {
		config.CacheTTLs.QuizDetail = "6h"
	}

	return config, nil
}

// ParseTTLStringOrDefault parses a TTL string (e.g., "1h", "30m") into a time.Duration.
// If parsing fails or the string is empty, it returns the defaultDuration.
func (c *Config) ParseTTLStringOrDefault(ttlString string, defaultDuration time.Duration) time.Duration {
	if ttlString == "" {
		return defaultDuration
	}
	duration, err := time.ParseDuration(ttlString)
	if err != nil {
		// In a real app, you'd use a proper logger here.
		// For now, printing to stdout for simplicity during refactoring.
		// This should be replaced with logger.Get().Warn(...)
		fmt.Printf("Warning: Failed to parse TTL string '%s', using default %v. Error: %v\n", ttlString, defaultDuration, err)
		return defaultDuration
	}
	return duration
}

func (c *Config) GetDSN() string {
	// Oracle DSN format: user/password@host:port/service
	return fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		c.DB.User,
		c.DB.Password,
		c.DB.Host,
		c.DB.Port,
		c.DB.DBName,
	)
}
