package validation

import (
	"quiz-byte/internal/domain"
	"regexp"
	"strings"
)

// Validator provides request validation functionality
type Validator struct{}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateCheckAnswerRequest validates the check answer request
func (v *Validator) ValidateCheckAnswerRequest(quizID, userAnswer string) domain.ValidationErrors {
	var errors domain.ValidationErrors

	if strings.TrimSpace(quizID) == "" {
		errors = append(errors, domain.NewMissingFieldError("quiz_id"))
	} else if !isValidULID(quizID) {
		errors = append(errors, domain.NewInvalidFormatError("quiz_id", quizID))
	}

	if strings.TrimSpace(userAnswer) == "" {
		errors = append(errors, domain.NewMissingFieldError("user_answer"))
	} else if len(userAnswer) > 2000 {
		errors = append(errors, domain.NewOutOfRangeError("user_answer", len(userAnswer), 1, 2000))
	}

	return errors
}

// ValidateSubCategory validates sub-category parameter
func (v *Validator) ValidateSubCategory(subCategory string) domain.ValidationErrors {
	var errors domain.ValidationErrors

	if strings.TrimSpace(subCategory) == "" {
		errors = append(errors, domain.NewMissingFieldError("sub_category"))
		return errors
	}

	// Check valid format (alphanumeric, hyphens, underscores)
	if !isValidSubCategory(subCategory) {
		errors = append(errors, domain.NewInvalidFormatError("sub_category", subCategory))
	}

	return errors
}

// ValidateBulkQuizzesRequest validates bulk quizzes request
func (v *Validator) ValidateBulkQuizzesRequest(subCategory string, count int) domain.ValidationErrors {
	var errors domain.ValidationErrors

	// Validate sub-category
	if subCatErrors := v.ValidateSubCategory(subCategory); len(subCatErrors) > 0 {
		errors = append(errors, subCatErrors...)
	}

	// Validate count
	if count <= 0 || count > 50 {
		errors = append(errors, domain.NewOutOfRangeError("count", count, 1, 50))
	}

	return errors
}

// Helper functions for validation

// isValidULID checks if the string is a valid ULID format
func isValidULID(s string) bool {
	// ULID is 26 characters long, base32 encoded
	if len(s) != 26 {
		return false
	}
	// Check if all characters are valid base32 (Crockford's Base32)
	validULID := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	return validULID.MatchString(s)
}

// isValidSubCategory checks if the sub-category format is valid
func isValidSubCategory(s string) bool {
	// Allow alphanumeric, hyphens, and underscores, 1-50 characters
	if len(s) == 0 || len(s) > 50 {
		return false
	}
	validSubCategory := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return validSubCategory.MatchString(s)
}
