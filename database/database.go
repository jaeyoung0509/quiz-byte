package database

import (
	"fmt"
	"os"

	_ "github.com/godror/godror" // Oracle driver
	"github.com/jmoiron/sqlx"
)

// InitDB initializes the Oracle database connection using Sqlx.
func InitDB() (*sqlx.DB, error) {
	// Read database connection details from environment variables
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbService := os.Getenv("DB_SERVICE_NAME") // Or DB_SID, depending on configuration

	if dbUser == "" || dbPassword == "" || dbHost == "" || dbPort == "" || dbService == "" {
		return nil, fmt.Errorf("database environment variables (DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_SERVICE_NAME) must be set")
	}

	// Construct the DSN (Data Source Name) for godror
	// Example DSN: user="dbUser" password="dbPassword" connectString="dbHost:dbPort/dbService"
	// Adjust based on your Oracle Naming Method (e.g., EZCONNECT, TNSNAMES)
	// For EZCONNECT: "dbHost:dbPort/dbService"
	// For TNSNAMES: "tns_alias" (ensure TNS_ADMIN is set or tnsnames.ora is in a default location)
	connectString := fmt.Sprintf("%s:%s/%s", dbHost, dbPort, dbService)
	dsn := fmt.Sprintf("user=\"%s\" password=\"%s\" connectString=\"%s\"", dbUser, dbPassword, connectString)

	// Open the database connection using sqlx
	db, err := sqlx.Connect("godror", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database using sqlx: %w", err)
	}

	// Ping the database to verify the connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Println("Database connection established successfully using Sqlx.")

	return db, nil
}
