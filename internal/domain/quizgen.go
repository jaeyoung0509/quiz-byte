package domain

import (
	"context"
	// NewQuizData is defined in internal/domain/batch.go, which is also package domain.
	// Therefore, it's directly accessible here.
)

// QuizGenerationService defines the interface for generating quiz candidates.
type QuizGenerationService interface {
	// GenerateQuizCandidates generates a specified number of quiz candidates
	// for a given subcategory, considering existing keywords to promote novelty.
	GenerateQuizCandidates(
		ctx context.Context,
		subCategoryName string,
		existingKeywords []string,
		numQuestions int,
	) ([]*NewQuizData, error)
	GenerateScoreEvaluationsForQuiz(ctx context.Context, quiz *Quiz, scoreRanges []string) ([]ScoreEvaluationDetail, error) // Added
}
