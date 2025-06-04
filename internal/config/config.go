package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB        DBConfig
	Server    ServerConfig
	LLMServer string
	Redis     RedisConfig
	Embedding EmbeddingConfig // New field
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
	}

	// Set default for SimilarityThreshold if not provided or zero
	if !viper.IsSet("embedding.similarity_threshold") || config.Embedding.SimilarityThreshold == 0 {
		config.Embedding.SimilarityThreshold = 0.95 // Default value
	}

	return config, nil
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
