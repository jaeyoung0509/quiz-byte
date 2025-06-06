package main

import (
	"fmt"
	"log"
	"os"

	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
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

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run -tags godror cmd/migrate/main.go <up|down|reset|clean|status>")
	}
	cmd := os.Args[1]

	var n int
	switch cmd {
	case "clean":
		// 모든 테이블을 강제로 삭제 (개발환경에서만 사용)
		fmt.Println("Cleaning all tables...")
		err = database.CleanDatabase(db)
		if err != nil {
			log.Fatalf("Failed to clean database: %v", err)
		}
		fmt.Println("Cleaned all tables and migration records")

	case "reset":
		// 데이터베이스 리셋 (clean + up)
		fmt.Println("Resetting database...")
		err = database.ResetDatabase(db)
		if err != nil {
			log.Fatalf("Failed to reset database: %v", err)
		}

	case "up":
		n, err = database.ApplyMigrations(db)
		if err != nil {
			log.Fatalf("Failed to apply migrations 'up': %v", err)
		}
		log.Printf("Applied %d migrations!\n", n)

	case "down":
		n, err = database.RollbackMigrations(db, 1)
		if err != nil {
			log.Fatalf("Failed to apply migration 'down': %v", err)
		}
		log.Printf("Rolled back %d migration!\n", n)

	case "status":
		// 마이그레이션 상태 확인
		records, err := database.GetMigrationStatus(db)
		if err != nil {
			log.Fatalf("Failed to get migration records: %v", err)
		}

		fmt.Printf("Migration Status:\n")
		for _, record := range records {
			fmt.Printf("- %s (applied at: %v)\n", record.Id, record.AppliedAt)
		}

	default:
		log.Fatalf("Unknown command: %s. Use 'up', 'down', 'reset', 'clean', or 'status'.", cmd)
	}
}
