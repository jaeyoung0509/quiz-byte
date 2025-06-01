package main

import (
	"log"
	"os"
	"quiz-byte/internal/database"
)

func main() {
	// Oracle DB connection string
	// Format: user/password@host:port/service_name
	dsn := os.Getenv("ORACLE_DSN")
	if dsn == "" {
		dsn = "system/oracle@localhost:1521/QUIZDB"
	}

	// DB connection
	db, err := database.NewMigrateOracleDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
}
