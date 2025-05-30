package domain

// AnswerEvaluator defines the interface for evaluating answers
type AnswerEvaluator interface {
	// EvaluateAnswer evaluates a user's answer to a quiz
	EvaluateAnswer(questionText string, modelAnswer string, userAnswer string, keywords []string) (*Answer, error)
}
