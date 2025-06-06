package port

import "quiz-byte/internal/domain"

// AnswerEvaluator defines the interface for evaluating user answers
type AnswerEvaluator interface {
	EvaluateAnswer(questionText string, modelAnswer string, userAnswer string, keywords []string) (*domain.Answer, error)
}
