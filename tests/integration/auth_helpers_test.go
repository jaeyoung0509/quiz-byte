package integration

import (
	"database/sql"
	"fmt"
	"time"

	"quiz-byte/internal/dto"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

// createTestUserDB creates a user in the database for testing purposes.
// The models.User input should have its ID pre-filled, typically with util.NewULID().
func createTestUserDB(db *sqlx.DB, user models.User) (*models.User, error) {
	if db == nil {
		return nil, fmt.Errorf("database instance is nil")
	}

	// Ensure all necessary fields are set, especially those with NOT NULL constraints
	// or default values in the schema if not provided by the 'user' struct.
	// For example, created_at and updated_at are often set by the DB or application logic.
	// Here, we assume they are part of the input 'user' model.
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = time.Now()
	}
	if user.ID == "" {
		return nil, fmt.Errorf("user ID must be pre-filled")
	}

	query := `INSERT INTO users (id, email, google_id, name, profile_picture_url, created_at, updated_at)
              VALUES (:id, :email, :google_id, :name, :profile_picture_url, :created_at, :updated_at)`
	_, err := db.NamedExec(query, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to insert test user: %w", err)
	}

	// Optionally, retrieve the user from DB to confirm insertion and get any DB-generated fields
	// For now, returning the input user as it was inserted.
	return &user, nil
}

// generateTestJWTToken generates a JWT token for a given user.
// It uses the global cfg variable (from main_test.go) for JWT secret and TTLs.
func generateTestJWTToken(user *models.User, tokenType string, ttl time.Duration) (string, error) {
	if cfg == nil || cfg.Auth.JWT.SecretKey == "" {
		return "", fmt.Errorf("JWT secret key is not configured in global cfg")
	}
	if user == nil {
		return "", fmt.Errorf("user cannot be nil")
	}

	issuedAt := time.Now()
	expiresAt := issuedAt.Add(ttl)

	claims := dto.AuthClaims{
		UserID:    user.ID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.Auth.JWT.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// Example usage for creating a test user:
func newTestUser() models.User {
	userID := util.NewULID()
	return models.User{
		ID:                userID,
		Email:             "testuser-" + userID + "@example.com",
		GoogleID:          "googleid-" + userID,
		Name:              sql.NullString{String: "Test User " + userID, Valid: true},
		ProfilePictureURL: sql.NullString{String: "http://example.com/picture-" + userID + ".jpg", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}
