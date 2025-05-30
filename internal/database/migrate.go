package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	// godror 드라이버를 사용하여 Oracle DB 연결
	db, err := sql.Open("godror", dsn)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %v", err)
	}

	// 연결 테스트
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not ping database: %v", err)
	}

	return db, nil
}
