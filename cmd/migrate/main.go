package main

import (
	"log"
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

	logger.Initialize(cfg)
	l := logger.Get()
	defer l.Sync()

	dsn := cfg.GetDSN()
	// DB connection
	db, err := database.NewMigrateOracleDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db, "../../database/migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
}
