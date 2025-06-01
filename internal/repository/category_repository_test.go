package repository

import (
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
func setupCategoryTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return sqlxDB, mock
}

func TestGetAllCategories(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryRepository(db)

	now := time.Now()
	expectedCategories := []models.Category{
		{ID: util.NewULID(), Name: "Category 1", Description: "Desc 1", CreatedAt: now, UpdatedAt: now},
		{ID: util.NewULID(), Name: "Category 2", Description: "Desc 2", CreatedAt: now, UpdatedAt: now},
	}

	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"})
	for _, cat := range expectedCategories {
		rows.AddRow(cat.ID, cat.Name, cat.Description, cat.CreatedAt, cat.UpdatedAt, nil)
	}

	query := `SELECT id, name, description, created_at, updated_at, deleted_at FROM categories WHERE deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)

	result, err := repo.GetAllCategories()

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
	repo := NewCategoryRepository(db)

	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"})

	query := `SELECT id, name, description, created_at, updated_at, deleted_at FROM categories WHERE deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)

	result, err := repo.GetAllCategories()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSubCategories(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryRepository(db)

	categoryID := util.NewULID()
	now := time.Now()
	expectedSubCategories := []models.SubCategory{
		{ID: util.NewULID(), CategoryID: categoryID, Name: "SubCat 1", Description: "SubDesc 1", CreatedAt: now, UpdatedAt: now},
		{ID: util.NewULID(), CategoryID: categoryID, Name: "SubCat 2", Description: "SubDesc 2", CreatedAt: now, UpdatedAt: now},
	}

	rows := sqlmock.NewRows([]string{"id", "category_id", "name", "description", "created_at", "updated_at", "deleted_at"})
	for _, subCat := range expectedSubCategories {
		rows.AddRow(subCat.ID, subCat.CategoryID, subCat.Name, subCat.Description, subCat.CreatedAt, subCat.UpdatedAt, nil)
	}

	// The query in GetSubCategories uses $1 for positional arg.
	query := `SELECT id, category_id, name, description, created_at, updated_at, deleted_at FROM sub_categories WHERE category_id = $1 AND deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(categoryID).WillReturnRows(rows)

	result, err := repo.GetSubCategories(categoryID)

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
	repo := NewCategoryRepository(db)
	categoryID := util.NewULID()

	rows := sqlmock.NewRows([]string{"id", "category_id", "name", "description", "created_at", "updated_at", "deleted_at"})

	query := `SELECT id, category_id, name, description, created_at, updated_at, deleted_at FROM sub_categories WHERE category_id = $1 AND deleted_at IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(categoryID).WillReturnRows(rows)

	result, err := repo.GetSubCategories(categoryID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}


func TestSaveCategory(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryRepository(db)

	domainCategory := &domain.Category{
		Name:        "Test Category",
		Description: "Test Description",
	}

	// sqlx likely converts named parameters to positional ones (?) before execution.
	query := `INSERT INTO categories (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(sqlmock.AnyArg(), domainCategory.Name, domainCategory.Description, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.SaveCategory(domainCategory)

	assert.NoError(t, err)
	assert.NotEmpty(t, domainCategory.ID)
	assert.NotZero(t, domainCategory.CreatedAt)
	assert.NotZero(t, domainCategory.UpdatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveSubCategory(t *testing.T) {
	db, mock := setupCategoryTestDB(t)
	repo := NewCategoryRepository(db)

	domainSubCategory := &domain.SubCategory{
		CategoryID:  util.NewULID(),
		Name:        "Test SubCategory",
		Description: "Test SubDescription",
	}

	// sqlx likely converts named parameters to positional ones (?) before execution.
	query := `INSERT INTO sub_categories (id, category_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(
			sqlmock.AnyArg(), // id
			domainSubCategory.CategoryID,
			domainSubCategory.Name,
			domainSubCategory.Description,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.SaveSubCategory(domainSubCategory)

	assert.NoError(t, err)
	assert.NotEmpty(t, domainSubCategory.ID)
	assert.NotZero(t, domainSubCategory.CreatedAt)
	assert.NotZero(t, domainSubCategory.UpdatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}
