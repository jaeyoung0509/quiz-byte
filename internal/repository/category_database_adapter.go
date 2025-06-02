package repository

import (
	"database/sql"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"time"

	"github.com/jmoiron/sqlx"
)

type CategoryDatabaseAdapter struct {
	db *sqlx.DB
}

// NewCategoryDatabaseAdapter creates a new instance of CategoryDatabaseAdapter
func NewCategoryDatabaseAdapter(db *sqlx.DB) domain.CategoryRepository {
	return &CategoryDatabaseAdapter{db: db}
}

// GetAllCategories returns all categories
func (r *CategoryDatabaseAdapter) GetAllCategories() ([]*domain.Category, error) {
	var categories []models.Category
	query := "SELECT id, name, description, created_at, updated_at, deleted_at FROM categories WHERE deleted_at IS NULL"
	err := r.db.Select(&categories, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*domain.Category{}, nil
		}
		return nil, err
	}

	domainCategories := make([]*domain.Category, len(categories))
	for i, category := range categories {
		domainCategories[i] = convertToDomainCategory(&category)
	}
	return domainCategories, nil
}

// GetSubCategories returns all subcategories for a given category
func (r *CategoryDatabaseAdapter) GetSubCategories(categoryID string) ([]*domain.SubCategory, error) {
	var subCategories []models.SubCategory
	query := "SELECT id, category_id, name, description, created_at, updated_at, deleted_at FROM sub_categories WHERE category_id = $1 AND deleted_at IS NULL"
	err := r.db.Select(&subCategories, query, categoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*domain.SubCategory{}, nil
		}
		return nil, err
	}

	domainSubCategories := make([]*domain.SubCategory, len(subCategories))
	for i, subCategory := range subCategories {
		domainSubCategories[i] = convertToDomainSubCategory(&subCategory)
	}
	return domainSubCategories, nil
}

// SaveCategory persists a new category
func (r *CategoryDatabaseAdapter) SaveCategory(category *domain.Category) error {
	modelCategory := convertToModelCategory(category)
	modelCategory.ID = util.NewULID()
	modelCategory.CreatedAt = time.Now()
	modelCategory.UpdatedAt = time.Now()

	query := `INSERT INTO categories (id, name, description, created_at, updated_at)
              VALUES (:id, :name, :description, :created_at, :updated_at)`
	_, err := r.db.NamedExec(query, modelCategory)
	if err != nil {
		return err
	}
	category.ID = modelCategory.ID
	category.CreatedAt = modelCategory.CreatedAt
	category.UpdatedAt = modelCategory.UpdatedAt
	return nil
}

// SaveSubCategory persists a new subcategory
func (r *CategoryDatabaseAdapter) SaveSubCategory(subCategory *domain.SubCategory) error {
	modelSubCategory := convertToModelSubCategory(subCategory)
	modelSubCategory.ID = util.NewULID()
	modelSubCategory.CreatedAt = time.Now()
	modelSubCategory.UpdatedAt = time.Now()

	query := `INSERT INTO sub_categories (id, category_id, name, description, created_at, updated_at)
              VALUES (:id, :category_id, :name, :description, :created_at, :updated_at)`
	_, err := r.db.NamedExec(query, modelSubCategory)
	if err != nil {
		return err
	}
	subCategory.ID = modelSubCategory.ID
	subCategory.CreatedAt = modelSubCategory.CreatedAt
	subCategory.UpdatedAt = modelSubCategory.UpdatedAt
	return nil
}

// Helper functions for converting between domain and model types
func convertToDomainCategory(category *models.Category) *domain.Category {
	return &domain.Category{
		ID:          category.ID,
		Name:        category.Name,
		Description: category.Description,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
		// SubCategories are not preloaded with SQLx in this manner
	}
}

func convertToModelCategory(category *domain.Category) *models.Category {
	return &models.Category{
		ID:          category.ID,
		Name:        category.Name,
		Description: category.Description,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

func convertToDomainSubCategory(subCategory *models.SubCategory) *domain.SubCategory {
	return &domain.SubCategory{
		ID:          subCategory.ID,
		CategoryID:  subCategory.CategoryID,
		Name:        subCategory.Name,
		Description: subCategory.Description,
		CreatedAt:   subCategory.CreatedAt,
		UpdatedAt:   subCategory.UpdatedAt,
	}
}

func convertToModelSubCategory(subCategory *domain.SubCategory) *models.SubCategory {
	return &models.SubCategory{
		ID:          subCategory.ID,
		CategoryID:  subCategory.CategoryID,
		Name:        subCategory.Name,
		Description: subCategory.Description,
		CreatedAt:   subCategory.CreatedAt,
		UpdatedAt:   subCategory.UpdatedAt,
	}
}
