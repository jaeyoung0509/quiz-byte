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
	OpenAIAPIKey string `yaml:"openai_api_key"`
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

	viper.AutomaticEnv()

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
		OpenAIAPIKey: viper.GetString("openai_api_key"),
	}

	// Override with environment variables if set
	if port := os.Getenv("DB_PORT"); port != "" {
		config.DB.Port = viper.GetInt("db.port")
	}
	if host := os.Getenv("DB_HOST"); host != "" {
		config.DB.Host = host
	}
	if user := os.Getenv("DB_USER"); user != "" {
		config.DB.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		config.DB.Password = password
	}
	if dbname := os.Getenv("DB_NAME"); dbname != "" {
		config.DB.DBName = dbname
	}
	if port := os.Getenv("SERVER_PORT"); port != "" {
		config.Server.Port = viper.GetInt("server.port")
	}
	if llmServer := os.Getenv("LLM_SERVER"); llmServer != "" {
		config.LLMServer = llmServer
	}
	if redisAddress := os.Getenv("REDIS_ADDRESS"); redisAddress != "" {
		config.Redis.Address = redisAddress
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		config.Redis.Password = redisPassword
	}
	// REDIS_DB environment variable can also be added if needed
	if openAIKey := os.Getenv("OPENAI_API_KEY"); openAIKey != "" {
		config.OpenAIAPIKey = openAIKey
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
