package main

import (
	"log"
	"os"
	"quiz-byte/internal/database"
)

func main() {
	// Oracle DB 연결 문자열
	// 형식: user/password@host:port/service_name
	dsn := os.Getenv("ORACLE_DSN")
	if dsn == "" {
		dsn = "system/oracle@localhost:1521/QUIZDB"
	}

	// DB 연결
	db, err := database.NewMigrateOracleDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 마이그레이션 실행
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
}
