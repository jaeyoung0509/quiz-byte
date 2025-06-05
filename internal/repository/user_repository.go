package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"time"

	"github.com/jmoiron/sqlx"
)

// sqlxUserRepository implements domain.UserRepository using sqlx.
type sqlxUserRepository struct {
	db *sqlx.DB
}

// NewSQLXUserRepository creates a new instance of sqlxUserRepository.
func NewSQLXUserRepository(db *sqlx.DB) domain.UserRepository {
	return &sqlxUserRepository{db: db}
}

func toDomainUser(modelUser *models.User) *domain.User {
	if modelUser == nil {
		return nil
	}
	var deletedAt *time.Time
	if modelUser.DeletedAt.Valid {
		deletedAt = &modelUser.DeletedAt.Time
	}
	return &domain.User{
		ID:                modelUser.ID,
		GoogleID:          modelUser.GoogleID,
		Email:             modelUser.Email,
		Name:              modelUser.Name.String,
		ProfilePictureURL: modelUser.ProfilePictureURL.String,
		CreatedAt:         modelUser.CreatedAt,
		UpdatedAt:         modelUser.UpdatedAt,
		DeletedAt:         deletedAt,
	}
}

func fromDomainUser(domainUser *domain.User) *models.User {
	if domainUser == nil {
		return nil
	}
	var deletedAt sql.NullTime
	if domainUser.DeletedAt != nil {
		deletedAt = util.TimeToNullTime(*domainUser.DeletedAt)
	}
	return &models.User{
		ID:                  domainUser.ID,
		GoogleID:            domainUser.GoogleID,
		Email:               domainUser.Email,
		Name:                util.StringToNullString(domainUser.Name),
		ProfilePictureURL:   util.StringToNullString(domainUser.ProfilePictureURL),
		CreatedAt:           domainUser.CreatedAt,
		UpdatedAt:           domainUser.UpdatedAt,
		DeletedAt:           deletedAt,
		// EncryptedAccessToken, EncryptedRefreshToken, TokenExpiresAt are not part of domain.User
		// They will be their zero values (sql.NullString, sql.NullTime)
	}
}

// CreateUser inserts a new user into the database.
func (r *sqlxUserRepository) CreateUser(ctx context.Context, domainUser *domain.User) error {
	modelUser := fromDomainUser(domainUser)
	// Ensure CreatedAt and UpdatedAt are set if not already by fromDomainUser (they are)
	// The original CreateUser in domain.NewUser sets them, so they should be propagated.
	// If by chance they are zero, set them.
	if modelUser.CreatedAt.IsZero() {
		modelUser.CreatedAt = time.Now()
	}
	if modelUser.UpdatedAt.IsZero() {
		modelUser.UpdatedAt = time.Now()
	}


	query := `INSERT INTO users (id, google_id, email, name, profile_picture_url, created_at, updated_at, deleted_at)
	          VALUES (:id, :google_id, :email, :name, :profile_picture_url, :created_at, :updated_at, :deleted_at)`
	// Note: Removed token fields from insert as they are not in domain.User

	_, err := r.db.NamedExecContext(ctx, query, modelUser)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByGoogleID retrieves a user by their Google ID.
func (r *sqlxUserRepository) GetUserByGoogleID(ctx context.Context, googleID string) (*domain.User, error) {
	var modelUser models.User
	query := `SELECT * FROM users WHERE google_id = :google_id AND deleted_at IS NULL`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query for GetUserByGoogleID: %w", err)
	}
	defer stmt.Close()

	args := map[string]interface{}{"google_id": googleID}
	err = stmt.GetContext(ctx, &modelUser, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil, nil for not found
		}
		return nil, fmt.Errorf("failed to get user by google_id: %w", err)
	}
	return toDomainUser(&modelUser), nil
}

// GetUserByID retrieves a user by their internal ID.
func (r *sqlxUserRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	var modelUser models.User
	query := `SELECT * FROM users WHERE id = :id AND deleted_at IS NULL`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query for GetUserByID: %w", err)
	}
	defer stmt.Close()

	args := map[string]interface{}{"id": userID}
	err = stmt.GetContext(ctx, &modelUser, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil, nil for not found
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return toDomainUser(&modelUser), nil
}

// UpdateUser updates an existing user's information based on the domain.User model.
// Note: This will only update fields present in domain.User. Tokens are not managed here.
func (r *sqlxUserRepository) UpdateUser(ctx context.Context, domainUser *domain.User) error {
	modelUser := fromDomainUser(domainUser)
	modelUser.UpdatedAt = time.Now() // Ensure UpdatedAt is set

	query := `UPDATE users SET
				email = :email,
	            name = :name,
	            profile_picture_url = :profile_picture_url,
	            updated_at = :updated_at,
				deleted_at = :deleted_at
	          WHERE id = :id AND deleted_at IS NULL`
	// Note: Removed token fields from update as they are not in domain.User.
	// Added deleted_at in SET clause for potential soft delete updates through this method if needed,
	// though typically soft deletes have dedicated methods. If domainUser.DeletedAt is nil,
	// modelUser.DeletedAt will be sql.NullTime{Valid:false}, preserving existing DB value if not changing.
	// If domainUser.DeletedAt is set, it will update the DB field.

	result, err := r.db.NamedExecContext(ctx, query, modelUser)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		// It's possible the user was not found, or the data was the same and no update occurred.
		// For update, sql.ErrNoRows might be misleading if data was same.
		// Consider fetching first or relying on service layer logic for "not found".
		// For now, returning an error if no rows affected, which is common.
		return fmt.Errorf("user not found or no changes made: %w", sql.ErrNoRows)
	}

	return nil
}
