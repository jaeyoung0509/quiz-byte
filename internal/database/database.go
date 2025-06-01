package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/sijms/go-ora/v2" // Oracle driver
)

func NewSQLXOracleDB(dsn string) (*sqlx.DB, error) {
	// sqlx.Connect를 사용하여 Oracle DB 연결
	// 드라이버 이름으로 "oracle" 또는 "go_ora" 등을 사용할 수 있습니다.
	// 실제 사용하는 드라이버에 따라 정확한 이름을 확인해야 합니다.
	// 여기서는 "oracle"을 가정합니다.
	db, err := sqlx.Connect("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Oracle database: %v", err)
	}

	// 연결 테스트
	// sqlx.DB는 *sql.DB를 임베드하므로, Ping() 메소드를 직접 사용할 수 있습니다.
	if err := db.Ping(); err != nil {
		// 연결 실패 시 DB 객체를 닫아주는 것이 좋습니다.
		db.Close()
		return nil, fmt.Errorf("failed to ping Oracle database: %v", err)
	}

	log.Println("Successfully connected to Oracle database")
	return db, nil
}
