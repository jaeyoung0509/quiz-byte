package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"quiz-byte/internal/repository/models" // Assuming models are in this path
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByGoogleID(ctx context.Context, googleID string) (*models.User, error)
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	// Consider GetUserByEmail if needed for specific checks, though GoogleID is primary for OAuth
}

// sqlxUserRepository implements UserRepository using sqlx.
type sqlxUserRepository struct {
	db *sqlx.DB
}

// NewSQLXUserRepository creates a new instance of sqlxUserRepository.
func NewSQLXUserRepository(db *sqlx.DB) UserRepository {
	return &sqlxUserRepository{db: db}
}

// CreateUser inserts a new user into the database.
func (r *sqlxUserRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (id, google_id, email, name, profile_picture_url, encrypted_access_token, encrypted_refresh_token, token_expires_at, created_at, updated_at)
	          VALUES (:id, :google_id, :email, :name, :profile_picture_url, :encrypted_access_token, :encrypted_refresh_token, :token_expires_at, :created_at, :updated_at)`

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		// TODO: Add more specific error handling for duplicate google_id or email if the DB driver supports it well.
		// For Oracle, this might be an ORA-00001 error.
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByGoogleID retrieves a user by their Google ID.
func (r *sqlxUserRepository) GetUserByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE google_id = :google_id AND deleted_at IS NULL`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query for GetUserByGoogleID: %w", err)
	}
	defer stmt.Close()

	args := map[string]interface{}{"google_id": googleID}
	err = stmt.GetContext(ctx, &user, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil, nil for not found, services can handle this
		}
		return nil, fmt.Errorf("failed to get user by google_id: %w", err)
	}
	return &user, nil
}

// GetUserByID retrieves a user by their internal ID.
func (r *sqlxUserRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE id = :id AND deleted_at IS NULL`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query for GetUserByID: %w", err)
	}
	defer stmt.Close()

	args := map[string]interface{}{"id": userID}
	err = stmt.GetContext(ctx, &user, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil, nil for not found
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &user, nil
}

// UpdateUser updates an existing user's information.
// This implementation updates all fields provided in the user struct.
// More granular updates (e.g., only tokens) might be needed.
func (r *sqlxUserRepository) UpdateUser(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()

	// Build the SET part of the query dynamically to only update fields that are meant to be updated.
	// For simplicity here, we update several common fields. A more robust solution might involve
	// checking which fields in the `user` struct are non-nil or using a map of fields to update.
	// For now, we'll assume that the service layer prepares the `user` object with the correct fields to update.

	// Example: Updating tokens, name, profile picture
	query := `UPDATE users SET
				email = :email,
	            name = :name,
	            profile_picture_url = :profile_picture_url,
	            encrypted_access_token = :encrypted_access_token,
	            encrypted_refresh_token = :encrypted_refresh_token,
	            token_expires_at = :token_expires_at,
	            updated_at = :updated_at
	          WHERE id = :id AND deleted_at IS NULL`

	result, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows // Or a custom error indicating user not found or not updated
	}

	return nil
}

// Helper function to build SET clause for updates might be useful for a more dynamic UpdateUser
// func buildUserUpdateSetClause(user *models.User, args map[string]interface{}) string {
// 	var setClauses []string
// 	if user.Email != "" { // Assuming Email is not nullable in struct if being updated
// 		setClauses = append(setClauses, "email = :email")
// 		args["email"] = user.Email
// 	}
// 	if user.Name.Valid {
// 		setClauses = append(setClauses, "name = :name")
// 		args["name"] = user.Name
// 	}
//   // ... other fields ...
// 	setClauses = append(setClauses, "updated_at = :updated_at")
// 	args["updated_at"] = user.UpdatedAt
//	return strings.Join(setClauses, ", ")
// }
