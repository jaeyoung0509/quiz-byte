package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"quiz-byte/internal/config"
	"quiz-byte/internal/database"

	migrate "github.com/rubenv/sql-migrate"
)

func main() {
	// 설정 로드
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// DB 연결
	db, err := database.NewSQLXOracleDB(cfg.GetDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 마이그레이션 소스 설정
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	var migrationsPath string
	if strings.Contains(wd, "cmd/migrate") {
		migrationsPath = filepath.Join(wd, "..", "..", "database", "migrations")
	} else {
		migrationsPath = filepath.Join(wd, "database", "migrations")
	}

	migrations := &migrate.FileMigrationSource{
		Dir: migrationsPath,
	}

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run -tags godror cmd/migrate/main.go <up|down|reset>")
	}
	cmd := os.Args[1]

	var n int
	switch cmd {
	case "clean":
		// 모든 테이블을 강제로 삭제 (개발환경에서만 사용)
		fmt.Println("Cleaning all tables...")
		cleanSQL := []string{
			// Triggers 삭제 (000001)
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER quiz_evaluations_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER answers_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER quizzes_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER sub_categories_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER categories_updated_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
			// 000002에서 추가된 트리거들 (실제 이름)
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER users_updated_at_trigger'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TRIGGER user_quiz_attempts_updated_at_trigger'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -4080 THEN RAISE; END IF; END;",

			// Indexes 삭제 (000001)
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_quiz_evaluations_quiz_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_answers_quiz_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_quizzes_difficulty'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_quizzes_sub_category_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_sub_categories_category_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			// 000002에서 추가된 인덱스들 (실제 이름)
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_user_quiz_attempts_user_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_user_quiz_attempts_quiz_id'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP INDEX idx_user_quiz_attempts_attempted_at'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 AND SQLCODE != -1418 THEN RAISE; END IF; END;",

			// Tables 삭제 (dependency 순서대로)
			"BEGIN EXECUTE IMMEDIATE 'DROP TABLE quiz_evaluations CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TABLE answers CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TABLE quizzes CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TABLE sub_categories CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
			"BEGIN EXECUTE IMMEDIATE 'DROP TABLE categories CASCADE CONSTRAINTS'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;",
			// 000002에서 추가된 테이블들 (실제 이름)
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
		fmt.Println("Cleaned all tables and migration records")

	case "reset":
		// 모든 마이그레이션을 롤백
		fmt.Println("Rolling back all migrations...")
		n, err = migrate.ExecMax(db.DB, "godror", migrations, migrate.Down, 0)
		if err != nil {
			log.Printf("Warning during rollback: %v", err)
		}
		fmt.Printf("Rolled back %d migrations\n", n)

		// 다시 마이그레이션 적용
		fmt.Println("Applying migrations...")
		n, err = migrate.Exec(db.DB, "godror", migrations, migrate.Up)
		if err != nil {
			log.Fatalf("Failed to apply migrations 'up': %v", err)
		}
		log.Printf("Applied %d migrations!\n", n)

	case "up":
		n, err = migrate.Exec(db.DB, "godror", migrations, migrate.Up)
		if err != nil {
			log.Fatalf("Failed to apply migrations 'up': %v", err)
		}
		log.Printf("Applied %d migrations!\n", n)

	case "down":
		n, err = migrate.ExecMax(db.DB, "godror", migrations, migrate.Down, 1)
		if err != nil {
			log.Fatalf("Failed to apply migration 'down': %v", err)
		}
		log.Printf("Rolled back %d migration!\n", n)

	case "status":
		// 마이그레이션 상태 확인
		records, err := migrate.GetMigrationRecords(db.DB, "godror")
		if err != nil {
			log.Fatalf("Failed to get migration records: %v", err)
		}

		fmt.Printf("Migration Status:\n")
		for _, record := range records {
			fmt.Printf("- %s (applied at: %v)\n", record.Id, record.AppliedAt)
		}

	default:
		log.Fatalf("Unknown command: %s. Use 'up', 'down', 'reset', or 'status'.", cmd)
	}
}
