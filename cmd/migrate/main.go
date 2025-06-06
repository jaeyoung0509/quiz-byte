package main

import (
	"log"
	"os"
	"path/filepath"
	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
	"quiz-byte/internal/logger"
)

func main() {
	// Oracle DB connection string
	// Format: user/password@host:port/service_name
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Initialize(cfg.Logger)
	l := logger.Get()
	defer l.Sync()

	dsn := cfg.GetDSN()
	// DB connection
	db, err := database.NewMigrateOracleDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Find migrations directory relative to current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	var migrationsPath string
	// Check if we're in the cmd/migrate directory
	if filepath.Base(wd) == "migrate" {
		migrationsPath = filepath.Join(wd, "..", "..", "database", "migrations")
	} else {
		// Assume we're in the project root
		migrationsPath = filepath.Join(wd, "database", "migrations")
	}

	// Run migrations
	if err := database.RunMigrations(db, migrationsPath); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
}
