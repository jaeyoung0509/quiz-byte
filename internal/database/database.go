package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/sijms/go-ora/v2" // Oracle driver
)

func NewSQLXOracleDB(dsn string) (*sqlx.DB, error) {
	// Connect to Oracle DB using sqlx.Connect
	// Driver name can be "oracle", "go_ora", etc.
	// You need to check the exact name depending on the driver you are using.
	// Here, "oracle" is assumed.
	db, err := sqlx.Connect("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Oracle database: %v", err)
	}

	// Test connection
	// sqlx.DB embeds *sql.DB, so Ping() method can be used directly.
	if err := db.Ping(); err != nil {
		// It is recommended to close the DB object upon connection failure.
		db.Close()
		return nil, fmt.Errorf("failed to ping Oracle database: %v", err)
	}

	log.Println("Successfully connected to Oracle database")
	return db, nil
}
