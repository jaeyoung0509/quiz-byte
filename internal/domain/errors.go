package domain

import (
	"errors"
	"fmt"
)

// Base domain errors following Go error conventions
var (
	// Common errors
	ErrInternal     = errors.New("internal error")
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")

	// Quiz specific errors
	ErrQuizNotFound    = errors.New("quiz not found")
	ErrInvalidAnswer   = errors.New("invalid answer")
	ErrLLMServiceError = errors.New("llm service error")
	ErrInvalidCategory = errors.New("invalid category")

	// Validation errors
	ErrValidation    = errors.New("validation error")
	ErrMissingField  = errors.New("missing required field")
	ErrInvalidFormat = errors.New("invalid format")
	ErrOutOfRange    = errors.New("value out of range")
)

// ErrorCode represents error classification for API responses
type ErrorCode string

const (
	CodeInternal     ErrorCode = "INTERNAL_ERROR"
	CodeInvalidInput ErrorCode = "INVALID_INPUT"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"

	CodeQuizNotFound    ErrorCode = "QUIZ_NOT_FOUND"
	CodeInvalidAnswer   ErrorCode = "INVALID_ANSWER"
	CodeLLMServiceError ErrorCode = "LLM_SERVICE_ERROR"
	CodeInvalidCategory ErrorCode = "INVALID_CATEGORY"

	CodeValidation    ErrorCode = "VALIDATION_ERROR"
	CodeMissingField  ErrorCode = "MISSING_FIELD"
	CodeInvalidFormat ErrorCode = "INVALID_FORMAT"
	CodeOutOfRange    ErrorCode = "OUT_OF_RANGE"
)

// DomainError represents a domain-specific error with additional context
type DomainError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]interface{}
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap allows errors.Is and errors.As to work properly
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for errors.Is
func (e *DomainError) Is(target error) bool {
	if e.Cause != nil {
		return errors.Is(e.Cause, target)
	}
	return false
}

// New creates a new DomainError with wrapping capabilities
func NewError(code ErrorCode, message string, cause error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *DomainError) WithContext(key string, value interface{}) *DomainError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Helper functions for common errors following Go conventions

// Wrap wraps an existing error with domain context
func WrapError(code ErrorCode, message string, err error) error {
	if err == nil {
		return nil
	}
	return NewError(code, message, err)
}

func NewNotFoundError(message string) error {
	return NewError(CodeNotFound, message, ErrNotFound)
}

func NewInvalidInputError(message string) error {
	return NewError(CodeInvalidInput, message, ErrInvalidInput)
}

func NewValidationError(message string) error {
	return NewError(CodeValidation, message, ErrValidation)
}

func NewInternalError(message string, err error) error {
	return NewError(CodeInternal, message, fmt.Errorf("%w: %v", ErrInternal, err))
}

func NewQuizNotFoundError(quizID string) error {
	return NewError(CodeQuizNotFound, fmt.Sprintf("Quiz not found with ID: %s", quizID), ErrQuizNotFound).
		WithContext("quiz_id", quizID)
}

func NewInvalidAnswerError(message string) error {
	return NewError(CodeInvalidAnswer, message, ErrInvalidAnswer)
}

func NewLLMServiceError(err error) error {
	return NewError(CodeLLMServiceError, "Failed to process with LLM service", fmt.Errorf("%w: %v", ErrLLMServiceError, err))
}

func NewInvalidCategoryError(category string) error {
	return NewError(CodeInvalidCategory, fmt.Sprintf("Invalid category: %s", category), ErrInvalidCategory).
		WithContext("category", category)
}

// ValidationError represents field-level validation errors
type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
	Code    ErrorCode   `json:"code"`
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field %s: %s", v.Field, v.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return "validation errors"
	}
	return fmt.Sprintf("validation failed: %s", v[0].Message)
}

// Helper functions for validation errors
func NewMissingFieldError(field string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: fmt.Sprintf("field %s is required", field),
		Code:    CodeMissingField,
	}
}

func NewInvalidFormatError(field string, value interface{}) ValidationError {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: fmt.Sprintf("field %s has invalid format", field),
		Code:    CodeInvalidFormat,
	}
}

func NewOutOfRangeError(field string, value interface{}, min, max interface{}) ValidationError {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: fmt.Sprintf("field %s value must be between %v and %v", field, min, max),
		Code:    CodeOutOfRange,
	}
}
