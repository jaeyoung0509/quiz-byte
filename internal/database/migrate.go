package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/sijms/go-ora/v2" // Ensure go-ora driver is registered
)

func RunMigrations(db *sql.DB) error {
	migrationsDir := "database/migrations"
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("could not read migrations directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, file.Name()))
		if err != nil {
			return fmt.Errorf("could not read migration file %s: %v", file.Name(), err)
		}

		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("could not execute migration %s: %v", file.Name(), err)
		}

		log.Printf("Executed migration: %s", file.Name())
	}

	log.Println("Migrations completed successfully")
	return nil
}

func NewMigrateOracleDB(dsn string) (*sql.DB, error) {
	// Connect to Oracle DB using go-ora driver
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %v", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not ping database: %v", err)
	}

	return db, nil
}
