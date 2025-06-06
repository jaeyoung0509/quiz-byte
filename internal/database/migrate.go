package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

// GetMigrationsPath returns the path to migration files
func GetMigrationsPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// cmd/migrate에서 실행되는 경우
	if filepath.Base(wd) == "migrate" {
		return filepath.Join(wd, "..", "..", "database", "migrations"), nil
	}

	// 프로젝트 루트에서 실행되는 경우
	migrationsPath := filepath.Join(wd, "database", "migrations")
	if _, err := os.Stat(migrationsPath); err == nil {
		return migrationsPath, nil
	}

	// 테스트에서 실행되는 경우 (tests/integration에서 실행)
	if filepath.Base(wd) == "integration" {
		return filepath.Join(wd, "..", "..", "database", "migrations"), nil
	}

	// tests 디렉토리에서 실행되는 경우
	if filepath.Base(wd) == "tests" {
		return filepath.Join(wd, "..", "database", "migrations"), nil
	}

	return migrationsPath, nil
}

// CreateMigrationSource creates a migration source from the migrations directory
func CreateMigrationSource() (*migrate.FileMigrationSource, error) {
	migrationsPath, err := GetMigrationsPath()
	if err != nil {
		return nil, err
	}

	return &migrate.FileMigrationSource{
		Dir: migrationsPath,
	}, nil
}

// CleanDatabase removes all database objects (tables, triggers, indexes)
func CleanDatabase(db *sqlx.DB) error {
	cleanSQL := []string{
		// Triggers 삭제 (000001)
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER quiz_evaluations_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER answers_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER quizzes_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER sub_categories_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER categories_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
		// 000002에서 추가된 트리거들
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER users_updated_at_trigger'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER user_quiz_attempts_updated_at_trigger'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",

		// Indexes 삭제 (000001)
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_quiz_evaluations_quiz_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_answers_quiz_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_quizzes_difficulty'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_quizzes_sub_category_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_sub_categories_category_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		// 000002에서 추가된 인덱스들
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_user_quiz_attempts_user_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_user_quiz_attempts_quiz_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_user_quiz_attempts_attempted_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",

		// Tables 삭제 (dependency 순서대로)
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE quiz_evaluations CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE answers CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE quizzes CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE sub_categories CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE categories CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
		// 000002에서 추가된 테이블들
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE user_quiz_attempts CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE users CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",

		// Migration table 삭제
		"BEGIN EXECUTE IMMEDIATE 'DROP TABLE gorp_migrations'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
	}

	for _, sql := range cleanSQL {
		_, err := db.Exec(sql)
		if err != nil {
			log.Printf("Warning during clean: %v", err)
		}
	}

	return nil
}

// ApplyMigrations applies all pending migrations
func ApplyMigrations(db *sqlx.DB) (int, error) {
	migrations, err := CreateMigrationSource()
	if err != nil {
		return 0, fmt.Errorf("failed to create migration source: %w", err)
	}

	n, err := migrate.Exec(db.DB, "godror", migrations, migrate.Up)
	if err != nil {
		return 0, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return n, nil
}

// RollbackMigrations rolls back n migrations
func RollbackMigrations(db *sqlx.DB, count int) (int, error) {
	migrations, err := CreateMigrationSource()
	if err != nil {
		return 0, fmt.Errorf("failed to create migration source: %w", err)
	}

	n, err := migrate.ExecMax(db.DB, "godror", migrations, migrate.Down, count)
	if err != nil {
		return 0, fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return n, nil
}

// RollbackAllMigrations rolls back all applied migrations
func RollbackAllMigrations(db *sqlx.DB) (int, error) {
	return RollbackMigrations(db, 0)
}

// ResetDatabase cleans the database and applies all migrations
func ResetDatabase(db *sqlx.DB) error {
	// Clean all database objects
	if err := CleanDatabase(db); err != nil {
		return fmt.Errorf("failed to clean database: %w", err)
	}

	// Apply all migrations
	n, err := ApplyMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to apply migrations after clean: %w", err)
	}

	log.Printf("Database reset completed. Applied %d migrations.", n)
	return nil
}

// GetMigrationStatus returns the current migration status
func GetMigrationStatus(db *sqlx.DB) ([]*migrate.MigrationRecord, error) {
	records, err := migrate.GetMigrationRecords(db.DB, "godror")
	if err != nil {
		return nil, fmt.Errorf("failed to get migration records: %w", err)
	}

	return records, nil
}

// InitializeDatabaseForTests initializes the database for testing by cleaning and applying migrations
func InitializeDatabaseForTests(db *sqlx.DB) error {
	log.Println("Initializing database for tests...")

	// Clean all existing database objects
	if err := CleanDatabase(db); err != nil {
		return fmt.Errorf("failed to clean database for tests: %w", err)
	}

	// Apply all migrations
	n, err := ApplyMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to apply migrations for tests: %w", err)
	}

	log.Printf("Database initialized for tests. Applied %d migrations.", n)
	return nil
}
