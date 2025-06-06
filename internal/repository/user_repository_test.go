package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// setupUserTestDB creates a new sqlx.DB instance and sqlmock for user repository testing.
func setupUserTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return sqlxDB, mock
}

// --- Tests for Converter Functions ---

func TestToDomainUser(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	modelUser := &models.User{
		ID:                "user1",
		GoogleID:          "google123",
		Email:             "test@example.com",
		Name:              sql.NullString{String: "Test User", Valid: true},
		ProfilePictureURL: sql.NullString{String: "http://example.com/pic.jpg", Valid: true},
		CreatedAt:         now,
		UpdatedAt:         now,
		DeletedAt:         sql.NullTime{},
	}

	domainUser := toDomainUser(modelUser)
	assert.NotNil(t, domainUser)
	assert.Equal(t, modelUser.ID, domainUser.ID)
	assert.Equal(t, modelUser.GoogleID, domainUser.GoogleID)
	assert.Equal(t, modelUser.Email, domainUser.Email)
	assert.Equal(t, modelUser.Name.String, domainUser.Name)
	assert.Equal(t, modelUser.ProfilePictureURL.String, domainUser.ProfilePictureURL)
	assert.True(t, modelUser.CreatedAt.Equal(domainUser.CreatedAt))
	assert.True(t, modelUser.UpdatedAt.Equal(domainUser.UpdatedAt))
	assert.Nil(t, domainUser.DeletedAt)

	// Test with NullString being null
	modelUser.Name.Valid = false
	modelUser.ProfilePictureURL.Valid = false
	domainUser = toDomainUser(modelUser)
	assert.NotNil(t, domainUser)
	assert.Equal(t, "", domainUser.Name)
	assert.Equal(t, "", domainUser.ProfilePictureURL)

	// Test with DeletedAt being valid
	deletedTime := now.Add(-time.Hour)
	modelUser.DeletedAt = sql.NullTime{Time: deletedTime, Valid: true}
	domainUser = toDomainUser(modelUser)
	assert.NotNil(t, domainUser)
	assert.NotNil(t, domainUser.DeletedAt)
	assert.True(t, deletedTime.Equal(*domainUser.DeletedAt))

	// Test nil input
	assert.Nil(t, toDomainUser(nil))
}

func TestFromDomainUser(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	domainUser := &domain.User{
		ID:                "user1",
		GoogleID:          "google123",
		Email:             "test@example.com",
		Name:              "Test User",
		ProfilePictureURL: "http://example.com/pic.jpg",
		CreatedAt:         now,
		UpdatedAt:         now,
		DeletedAt:         nil,
	}

	modelUser := fromDomainUser(domainUser)
	assert.NotNil(t, modelUser)
	assert.Equal(t, domainUser.ID, modelUser.ID)
	assert.Equal(t, domainUser.GoogleID, modelUser.GoogleID)
	assert.Equal(t, domainUser.Email, modelUser.Email)
	assert.Equal(t, domainUser.Name, modelUser.Name.String)
	assert.True(t, modelUser.Name.Valid)
	assert.Equal(t, domainUser.ProfilePictureURL, modelUser.ProfilePictureURL.String)
	assert.True(t, modelUser.ProfilePictureURL.Valid)
	assert.True(t, domainUser.CreatedAt.Equal(modelUser.CreatedAt))
	assert.True(t, domainUser.UpdatedAt.Equal(modelUser.UpdatedAt))
	assert.False(t, modelUser.DeletedAt.Valid)

	// Test with empty strings for nullable fields
	domainUser.Name = ""
	domainUser.ProfilePictureURL = ""
	modelUser = fromDomainUser(domainUser)
	assert.NotNil(t, modelUser)
	assert.Equal(t, "", modelUser.Name.String)
	assert.False(t, modelUser.Name.Valid) // util.StringToNullString makes it invalid if empty
	assert.Equal(t, "", modelUser.ProfilePictureURL.String)
	assert.False(t, modelUser.ProfilePictureURL.Valid)

	// Test with DeletedAt being set
	deletedTime := now.Add(-time.Hour)
	domainUser.DeletedAt = &deletedTime
	modelUser = fromDomainUser(domainUser)
	assert.NotNil(t, modelUser)
	assert.True(t, modelUser.DeletedAt.Valid)
	assert.True(t, deletedTime.Equal(modelUser.DeletedAt.Time))

	// Test nil input
	assert.Nil(t, fromDomainUser(nil))
}

// --- Tests for Adapter Methods ---

func TestSQLXUserRepository_GetUserByID_Success(t *testing.T) {
	db, mock := setupUserTestDB(t)
	repo := NewSQLXUserRepository(db) // Use the actual constructor
	defer db.Close()

	userID := "user-test-id"
	now := time.Now()
	expectedModel := models.User{
		ID:        userID,
		GoogleID:  "google-id",
		Email:     "test@example.com",
		Name:      sql.NullString{String: "Test User", Valid: true},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// For sqlx, column names in Rows must match struct fields or `db` tags.
	rows := sqlmock.NewRows([]string{"id", "google_id", "email", "name", "profile_picture_url", "encrypted_access_token", "encrypted_refresh_token", "token_expires_at", "created_at", "updated_at", "deleted_at"}).
		AddRow(expectedModel.ID, expectedModel.GoogleID, expectedModel.Email, expectedModel.Name, expectedModel.ProfilePictureURL, nil, nil, nil, expectedModel.CreatedAt, expectedModel.UpdatedAt, nil)

	// The query in GetUserByID uses named arg :id. sqlx rebinds this.
	// The PrepareNamedContext is used, so the query passed to Prepare is the original one.
	// Then GetContext uses this prepared statement.
	mock.ExpectPrepare(`SELECT \* FROM users WHERE id = :id AND deleted_at IS NULL`).
		ExpectQuery(). // This refers to the execution of the prepared statement
		WithArgs(userID).
		WillReturnRows(rows)

	domainUser, err := repo.GetUserByID(context.Background(), userID)

	assert.NoError(t, err)
	assert.NotNil(t, domainUser)
	assert.Equal(t, expectedModel.ID, domainUser.ID)
	assert.Equal(t, expectedModel.Email, domainUser.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLXUserRepository_GetUserByID_NotFound(t *testing.T) {
	db, mock := setupUserTestDB(t)
	repo := NewSQLXUserRepository(db)
	defer db.Close()

	userID := "non-existent-id"

	mock.ExpectPrepare(`SELECT \* FROM users WHERE id = :id AND deleted_at IS NULL`).
		ExpectQuery().
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	domainUser, err := repo.GetUserByID(context.Background(), userID)

	// Adapter returns (nil, nil) for sql.ErrNoRows from GetContext
	assert.NoError(t, err, "Expected no error from adapter when record not found")
	assert.Nil(t, domainUser, "Expected nil user for not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLXUserRepository_CreateUser_Success(t *testing.T) {
	db, mock := setupUserTestDB(t)
	repo := NewSQLXUserRepository(db)
	defer db.Close()

	domainUser := &domain.User{
		ID:       "new-user-id", // Assuming ID is set before calling CreateUser for predictability
		GoogleID: "new-google-id",
		Email:    "new@example.com",
		Name:     "New User",
		// CreatedAt and UpdatedAt will be set by the method if zero, or use provided.
	}

	// Query uses named exec. sqlx rebinds this.
	// Regex matches the query structure.
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users (id, google_id, email, name, profile_picture_url, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)).
		WillReturnResult(sqlmock.NewResult(1, 1)) // Args are checked by sqlmock with NamedExec

	err := repo.CreateUser(context.Background(), domainUser)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
