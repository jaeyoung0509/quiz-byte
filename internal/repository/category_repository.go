package repository

import (
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"

	"gorm.io/gorm"
)

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository creates a new instance of CategoryRepository
func NewCategoryRepository(db *gorm.DB) domain.CategoryRepository {
	return &categoryRepository{db: db}
}

// GetAllCategories returns all categories
func (r *categoryRepository) GetAllCategories() ([]*domain.Category, error) {
	var categories []models.Category
	result := r.db.Preload("SubCategories").Find(&categories)
	if result.Error != nil {
		return nil, result.Error
	}

	domainCategories := make([]*domain.Category, len(categories))
	for i, category := range categories {
		domainCategories[i] = convertToDomainCategory(&category)
	}
	return domainCategories, nil
}

// GetSubCategories returns all subcategories for a given category
func (r *categoryRepository) GetSubCategories(categoryID int64) ([]*domain.SubCategory, error) {
	var subCategories []models.SubCategory
	result := r.db.Where("category_id = ?", categoryID).Find(&subCategories)
	if result.Error != nil {
		return nil, result.Error
	}

	domainSubCategories := make([]*domain.SubCategory, len(subCategories))
	for i, subCategory := range subCategories {
		domainSubCategories[i] = convertToDomainSubCategory(&subCategory)
	}
	return domainSubCategories, nil
}

// SaveCategory persists a new category
func (r *categoryRepository) SaveCategory(category *domain.Category) error {
	modelCategory := convertToModelCategory(category)
	result := r.db.Create(modelCategory)
	if result.Error != nil {
		return result.Error
	}
	category.ID = modelCategory.ID
	return nil
}

// SaveSubCategory persists a new subcategory
func (r *categoryRepository) SaveSubCategory(subCategory *domain.SubCategory) error {
	modelSubCategory := convertToModelSubCategory(subCategory)
	result := r.db.Create(modelSubCategory)
	if result.Error != nil {
		return result.Error
	}
	subCategory.ID = modelSubCategory.ID
	return nil
}

// Helper functions for converting between domain and model types
func convertToDomainCategory(category *models.Category) *domain.Category {
	subCategories := make([]*domain.SubCategory, len(category.SubCategories))
	for i, subCategory := range category.SubCategories {
		subCategories[i] = convertToDomainSubCategory(&subCategory)
	}

	return &domain.Category{
		ID:            category.ID,
		Name:          category.Name,
		Description:   category.Description,
		CreatedAt:     category.CreatedAt,
		UpdatedAt:     category.UpdatedAt,
		SubCategories: subCategories,
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
