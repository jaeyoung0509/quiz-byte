package domain

import (
	"encoding/json"
	"fmt"
)

// ErrorCode represents a specific type of error in the domain
type ErrorCode string

const (
	// Common errors
	ErrInternal     ErrorCode = "INTERNAL_ERROR"
	ErrInvalidInput ErrorCode = "INVALID_INPUT"
	ErrNotFound     ErrorCode = "NOT_FOUND"
	ErrUnauthorized ErrorCode = "UNAUTHORIZED"

	// Quiz specific errors
	ErrQuizNotFound    ErrorCode = "QUIZ_NOT_FOUND"
	ErrInvalidAnswer   ErrorCode = "INVALID_ANSWER"
	ErrLLMServiceError ErrorCode = "LLM_SERVICE_ERROR"
	ErrInvalidCategory ErrorCode = "INVALID_CATEGORY"
)

// DomainError represents a domain-specific error
type DomainError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// MarshalJSON implements the json.Marshaler interface
func (e *DomainError) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    string(e.Code),
		Message: e.Message,
	})
}

// New creates a new DomainError
func NewError(code ErrorCode, message string, err error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Helper functions for common errors
func NewNotFoundError(message string) *DomainError {
	return NewError(ErrNotFound, message, nil)
}

func NewInvalidInputError(message string) *DomainError {
	return NewError(ErrInvalidInput, message, nil)
}

func NewInternalError(message string, err error) *DomainError {
	return NewError(ErrInternal, message, err)
}

func NewQuizNotFoundError(quizID int64) *DomainError {
	return NewError(ErrQuizNotFound, fmt.Sprintf("Quiz not found with ID: %d", quizID), nil)
}

func NewInvalidAnswerError(message string) *DomainError {
	return NewError(ErrInvalidAnswer, message, nil)
}

func NewLLMServiceError(err error) *DomainError {
	return NewError(ErrLLMServiceError, "Failed to process with LLM service", err)
}

func NewInvalidCategoryError(category string) *DomainError {
	return NewError(ErrInvalidCategory, fmt.Sprintf("Invalid category: %s", category), nil)
}
