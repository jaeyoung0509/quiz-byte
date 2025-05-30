package database

import (
	"fmt"
	"log"

	oracle "github.com/godoes/gorm-oracle"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewOracleDB(dsn string) (*gorm.DB, error) {
	// GORM Oracle 드라이버를 사용하여 Oracle DB 연결
	// gorm.Open의 첫 번째 인자로 oracle.Open(dsn)을 전달합니다.
	db, err := gorm.Open(oracle.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		// Dialector 필드는 gorm.Open의 첫 번째 인자로 드라이버를 전달하므로 여기서는 필요 없습니다.
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Oracle database: %v", err)
	}

	// 연결 테스트
	sqlDB, err := db.DB() // gorm.DB에서 *sql.DB 객체 가져오기
	if err != nil {
		// 이 오류는 GORM v1에서는 발생할 수 있으나, GORM v2 (최신)에서는 db.DB()가 항상 *sql.DB와 error를 반환합니다.
		// GORM 버전 확인 필요. 최신 GORM이면 이 에러 핸들링은 맞습니다.
		return nil, fmt.Errorf("failed to get underlying database object: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Oracle database: %v", err)
	}

	log.Println("Successfully connected to Oracle database")
	return db, nil
}
