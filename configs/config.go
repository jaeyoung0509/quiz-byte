package configs

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App struct {
		Name    string
		Version string
		Port    string
	}
	Database struct {
		DSN string
	}
	LLM struct {
		ServerURL string
		Timeout   time.Duration
	}
}

func Load() (*Config, error) {
	config := &Config{}

	// App 설정
	config.App.Name = getEnv("APP_NAME", "quiz-byte")
	config.App.Version = getEnv("APP_VERSION", "0.1.0")
	config.App.Port = getEnv("PORT", "8080")

	// Database 설정
	config.Database.DSN = getEnv("ORACLE_DSN", "system/oracle@localhost:1521/QUIZDB")

	// LLM 설정
	config.LLM.ServerURL = getEnv("LLAMA_SERVER", "http://localhost:8080")
	timeout, err := strconv.Atoi(getEnv("LLM_TIMEOUT", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid LLM_TIMEOUT: %v", err)
	}
	config.LLM.Timeout = time.Duration(timeout) * time.Second

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
