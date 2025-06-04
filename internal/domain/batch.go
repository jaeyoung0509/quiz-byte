package domain

import "context"

// NewQuizData represents the data expected from the LLM.
type NewQuizData struct {
	Question    string
	ModelAnswer string
	Keywords    []string
	Difficulty  string
}

// BatchService defines the interface for batch operations.
type BatchService interface {
	GenerateNewQuizzesAndSave(ctx context.Context) error
}
