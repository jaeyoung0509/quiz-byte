package database

import (
	"fmt"
	"os"

	oracle "github.com/godoes/gorm-oracle"
	"gorm.io/gorm"
)

// InitDB initializes the Oracle database connection using GORM.
func InitDB() (*gorm.DB, error) {
	// Read database connection details from environment variables
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbService := os.Getenv("DB_SERVICE_NAME") // Or DB_SID, depending on configuration

	if dbUser == "" || dbPassword == "" || dbHost == "" || dbPort == "" || dbService == "" {
		return nil, fmt.Errorf("database environment variables (DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_SERVICE_NAME) must be set")
	}

	// Construct the DSN (Data Source Name)
	// Using the format recommended by the godoes/gorm-oracle driver:
	// oracle://user:password@host:port/service
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%s/%s", dbUser, dbPassword, dbHost, dbPort, dbService)

	// You might need additional URL options depending on your Oracle setup,
	// e.g., wallet path, SSL options, etc. These can also be read from env vars.
	// For example:
	// walletPath := os.Getenv("DB_WALLET_PATH")
	// if walletPath != "" {
	// 	dsn = fmt.Sprintf("%s?wallet=%s", dsn, walletPath)
	// }

	// Open the database connection
	db, err := gorm.Open(oracle.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Optional: Ping the database to verify the connection
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get generic database object: %w", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Println("Database connection established successfully.")

	return db, nil
}
