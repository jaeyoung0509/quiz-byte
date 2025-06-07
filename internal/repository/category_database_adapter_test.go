package repository

import (
	"context" // Added context
	"database/sql"
	"regexp"
	"testing"
	"time"

	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// setupCategoryTestDB creates a new sqlx.DB instance and sqlmock for category repository testing.
// Note: This is identical to setupTestDB in quiz_test.go. Consider moving to a shared test helper package if more repos are added.
func setupCategoryTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) { //nolint:thelper // This is a helper, but testify doesn't require *testing.T as first arg for helpers.
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return sqlxDB, mock
}

func TestGetAllCategories(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryDatabaseAdapter(db)

	now := time.Now()
	expectedCategories := []models.Category{
		{ID: util.NewULID(), Name: "Category 1", Description: sql.NullString{String: "Desc 1", Valid: true}, CreatedAt: now, UpdatedAt: now},
		{ID: util.NewULID(), Name: "Category 2", Description: sql.NullString{String: "Desc 2", Valid: true}, CreatedAt: now, UpdatedAt: now},
	}

	// Column names for sqlmock.NewRows should be uppercase to match struct fields/tags if sqlx maps to them.
	rows := sqlmock.NewRows([]string{"ID", "NAME", "DESCRIPTION", "CREATED_AT", "UPDATED_AT", "DELETED_AT"})
	for _, cat := range expectedCategories {
		rows.AddRow(cat.ID, cat.Name, cat.Description, cat.CreatedAt, cat.UpdatedAt, nil)
	}

	// Query remains non-aliased as per the actual adapter code.
	query := `SELECT id, name, description, created_at, updated_at, deleted_at FROM categories WHERE deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)

	result, err := repo.GetAllCategories(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, len(expectedCategories))
	for i, resCat := range result {
		assert.Equal(t, expectedCategories[i].ID, resCat.ID)
		assert.Equal(t, expectedCategories[i].Name, resCat.Name)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllCategories_Empty(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryDatabaseAdapter(db)

	// Column names for sqlmock.NewRows should be uppercase.
	rows := sqlmock.NewRows([]string{"ID", "NAME", "DESCRIPTION", "CREATED_AT", "UPDATED_AT", "DELETED_AT"})

	// Query remains non-aliased.
	query := `SELECT id, name, description, created_at, updated_at, deleted_at FROM categories WHERE deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)

	result, err := repo.GetAllCategories(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSubCategories(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryDatabaseAdapter(db)

	categoryID := util.NewULID()
	now := time.Now()
	expectedSubCategories := []models.SubCategory{
		{ID: util.NewULID(), CategoryID: categoryID, Name: "SubCat 1", Description: sql.NullString{String: "SubDesc 1", Valid: true}, CreatedAt: now, UpdatedAt: now},
		{ID: util.NewULID(), CategoryID: categoryID, Name: "SubCat 2", Description: sql.NullString{String: "SubDesc 2", Valid: true}, CreatedAt: now, UpdatedAt: now},
	}

	// Column names for sqlmock.NewRows should be uppercase.
	rows := sqlmock.NewRows([]string{"ID", "CATEGORY_ID", "NAME", "DESCRIPTION", "CREATED_AT", "UPDATED_AT", "DELETED_AT"})
	for _, subCat := range expectedSubCategories {
		rows.AddRow(subCat.ID, subCat.CategoryID, subCat.Name, subCat.Description, subCat.CreatedAt, subCat.UpdatedAt, nil)
	}

	// Query remains non-aliased.
	query := `SELECT id, category_id, name, description, created_at, updated_at, deleted_at FROM sub_categories WHERE category_id = :1 AND deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(categoryID).WillReturnRows(rows)

	result, err := repo.GetSubCategories(context.Background(), categoryID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, len(expectedSubCategories))
	for i, resSubCat := range result {
		assert.Equal(t, expectedSubCategories[i].ID, resSubCat.ID)
		assert.Equal(t, expectedSubCategories[i].Name, resSubCat.Name)
		assert.Equal(t, expectedSubCategories[i].CategoryID, resSubCat.CategoryID)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSubCategories_Empty(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryDatabaseAdapter(db)
	categoryID := util.NewULID()

	// Column names for sqlmock.NewRows should be uppercase.
	rows := sqlmock.NewRows([]string{"ID", "CATEGORY_ID", "NAME", "DESCRIPTION", "CREATED_AT", "UPDATED_AT", "DELETED_AT"})

	// Query remains non-aliased.
	query := `SELECT id, category_id, name, description, created_at, updated_at, deleted_at FROM sub_categories WHERE category_id = :1 AND deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(categoryID).WillReturnRows(rows)

	result, err := repo.GetSubCategories(context.Background(), categoryID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveCategory(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryDatabaseAdapter(db)

	domainCategory := &domain.Category{
		Name:        "Test Category",
		Description: "Test Description",
	}

	// sqlx likely converts named parameters to positional ones (?) before execution.
	// For sqlmock, the query string in ExpectExec should match what the driver receives.
	// If using named exec (:name), sqlx prepares a query with placeholders the driver understands (e.g., $1, $2 or ?).
	// It's often safer to use QueryMatcherEqual or ensure the regex is flexible.
	// Here, we assume the rebind results in a query sqlmock can match with regex for these placeholders.
	// The actual query sent to Oracle might be slightly different if named args are passed differently by the driver.
	mock.ExpectExec(`INSERT INTO categories \(id, name, description, created_at, updated_at\) VALUES \((?P<id>.*), (?P<name>.*), (?P<description>.*), (?P<created_at>.*), (?P<updated_at>.*)\)`).
		WithArgs(sqlmock.AnyArg(), domainCategory.Name, domainCategory.Description, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.SaveCategory(context.Background(), domainCategory)

	assert.NoError(t, err)
	assert.NotEmpty(t, domainCategory.ID)
	assert.NotZero(t, domainCategory.CreatedAt)
	assert.NotZero(t, domainCategory.UpdatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveSubCategory(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryDatabaseAdapter(db)

	domainSubCategory := &domain.SubCategory{
		CategoryID:  util.NewULID(),
		Name:        "Test SubCategory",
		Description: "Test SubDescription",
	}

	mock.ExpectExec(`INSERT INTO sub_categories \(id, category_id, name, description, created_at, updated_at\) VALUES \((?P<id>.*), (?P<category_id>.*), (?P<name>.*), (?P<description>.*), (?P<created_at>.*), (?P<updated_at>.*)\)`).
		WithArgs(
			sqlmock.AnyArg(), // id
			domainSubCategory.CategoryID,
			domainSubCategory.Name,
			domainSubCategory.Description,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.SaveSubCategory(context.Background(), domainSubCategory)

	assert.NoError(t, err)
	assert.NotEmpty(t, domainSubCategory.ID)
	assert.NotZero(t, domainSubCategory.CreatedAt)
	assert.NotZero(t, domainSubCategory.UpdatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConvertToDomainCategory(t *testing.T) {
	now := time.Now()
	model := &models.Category{
		ID:          "cat1",
		Name:        "Category 1",
		Description: sql.NullString{String: "Description 1", Valid: true},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	domainCat := convertToDomainCategory(model)
	assert.NotNil(t, domainCat)
	assert.Equal(t, model.ID, domainCat.ID)
	assert.Equal(t, model.Name, domainCat.Name)
	if model.Description.Valid {
		assert.Equal(t, model.Description.String, domainCat.Description)
	} else {
		assert.Equal(t, "", domainCat.Description) // Assuming NULL description converts to empty string
	}
	assert.True(t, model.CreatedAt.Equal(domainCat.CreatedAt)) // Use Equal for time
	assert.True(t, model.UpdatedAt.Equal(domainCat.UpdatedAt)) // Use Equal for time

	// Test nil input
	assert.Nil(t, convertToDomainCategory(nil), "Converting nil model should return nil domain category")
}

func TestConvertToModelCategory(t *testing.T) {
	now := time.Now()
	domainCat := &domain.Category{
		ID:          "cat1",
		Name:        "Category 1",
		Description: "Description 1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	model := convertToModelCategory(domainCat)
	assert.NotNil(t, model)
	assert.Equal(t, domainCat.ID, model.ID)
	assert.Equal(t, domainCat.Name, model.Name)
	if domainCat.Description == "" {
		assert.False(t, model.Description.Valid)
	} else {
		assert.True(t, model.Description.Valid)
		assert.Equal(t, domainCat.Description, model.Description.String)
	}
	assert.True(t, domainCat.CreatedAt.Equal(model.CreatedAt))
	assert.True(t, domainCat.UpdatedAt.Equal(model.UpdatedAt))

	// Test nil input
	assert.Nil(t, convertToModelCategory(nil), "Converting nil domain category should return nil model")
}

func TestConvertToDomainSubCategory(t *testing.T) {
	now := time.Now()
	model := &models.SubCategory{
		ID:          "subcat1",
		CategoryID:  "cat1",
		Name:        "SubCategory 1",
		Description: sql.NullString{String: "SubDescription 1", Valid: true},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	domainSubCat := convertToDomainSubCategory(model)
	assert.NotNil(t, domainSubCat)
	assert.Equal(t, model.ID, domainSubCat.ID)
	assert.Equal(t, model.CategoryID, domainSubCat.CategoryID)
	assert.Equal(t, model.Name, domainSubCat.Name)
	if model.Description.Valid {
		assert.Equal(t, model.Description.String, domainSubCat.Description)
	} else {
		assert.Equal(t, "", domainSubCat.Description) // Assuming NULL description converts to empty string
	}
	assert.True(t, model.CreatedAt.Equal(domainSubCat.CreatedAt))
	assert.True(t, model.UpdatedAt.Equal(domainSubCat.UpdatedAt))

	// Test nil input
	assert.Nil(t, convertToDomainSubCategory(nil), "Converting nil model subcategory should return nil domain subcategory")
}

func TestConvertToModelSubCategory(t *testing.T) {
	now := time.Now()
	domainSubCat := &domain.SubCategory{
		ID:          "subcat1",
		CategoryID:  "cat1",
		Name:        "SubCategory 1",
		Description: "SubDescription 1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	model := convertToModelSubCategory(domainSubCat)
	assert.NotNil(t, model)
	assert.Equal(t, domainSubCat.ID, model.ID)
	assert.Equal(t, domainSubCat.CategoryID, model.CategoryID)
	assert.Equal(t, domainSubCat.Name, model.Name)
	if domainSubCat.Description == "" {
		assert.False(t, model.Description.Valid)
	} else {
		assert.True(t, model.Description.Valid)
		assert.Equal(t, domainSubCat.Description, model.Description.String)
	}
	assert.True(t, domainSubCat.CreatedAt.Equal(model.CreatedAt))
	assert.True(t, domainSubCat.UpdatedAt.Equal(model.UpdatedAt))

	// Test nil input
	assert.Nil(t, convertToModelSubCategory(nil), "Converting nil domain subcategory should return nil model")
}
