package domain

import (
	"fmt" // For error messages if needed
	"testing"
	"time" // Required for NewQuizEvaluation if it sets times
)

func TestScoreEvaluationDetail_Validate_Dummy(t *testing.T) {
	// This struct has no Validate method, but we can test its creation.
	detail := ScoreEvaluationDetail{
		ScoreRange:    "0.8-1.0",
		SampleAnswers: []string{"Answer 1"},
		Explanation:   "Good job",
	}
	if detail.ScoreRange == "" {
		t.Error("ScoreRange should not be empty")
	}
}

func TestQuizEvaluation_Validate(t *testing.T) {
	validScoreRanges := []string{"0.0-0.5", "0.5-1.0"}
	validScoreEvals := []ScoreEvaluationDetail{
		{ScoreRange: "0.0-0.5", SampleAnswers: []string{"ans1"}, Explanation: "exp1"},
		{ScoreRange: "0.5-1.0", SampleAnswers: []string{"ans2"}, Explanation: "exp2"},
	}
	// validQuizEval := NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", validScoreEvals)
	// Create a valid one directly for baseline
	baseValidEval := func() *QuizEvaluation {
		return NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", validScoreEvals)
	}

	tests := []struct {
		name    string
		eval    *QuizEvaluation
		wantErr bool
		errText string // Optional: to check for specific error text
	}{
		{"valid evaluation", baseValidEval(), false, ""},
		{
			"missing quizID",
			NewQuizEvaluation("", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", validScoreEvals),
			true, "quiz ID is required",
		},
		{
			"missing score ranges",
			NewQuizEvaluation("q1", 1, []string{"topic"}, []string{}, []string{"sa"}, "rubric", []ScoreEvaluationDetail{}),
			true, "score ranges are required",
		},
		{
			"score evals count mismatch",
			NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", []ScoreEvaluationDetail{validScoreEvals[0]}),
			true, "score evaluations must correspond to defined score ranges in count",
		},
		{
			"score eval detail missing range",
			NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", []ScoreEvaluationDetail{
				{ScoreRange: "", SampleAnswers: []string{"ans1"}, Explanation: "exp1"},
				validScoreEvals[1],
			}),
			true, "score range in score evaluations is required",
		},
		{
			"score eval detail missing sample answers",
			NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", []ScoreEvaluationDetail{
				{ScoreRange: "0.0-0.5", SampleAnswers: []string{}, Explanation: "exp1"},
				validScoreEvals[1],
			}),
			true, "sample answers in score evaluation for range '0.0-0.5' are required",
		},
		{
			"score eval detail missing explanation",
			NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", []ScoreEvaluationDetail{
				{ScoreRange: "0.0-0.5", SampleAnswers: []string{"ans1"}, Explanation: ""},
				validScoreEvals[1],
			}),
			true, "explanation in score evaluation for range '0.0-0.5' is required",
		},
		{
			"score eval detail range not in main ScoreRanges",
			NewQuizEvaluation("q1", 1, []string{"topic"}, validScoreRanges, []string{"sa"}, "rubric", []ScoreEvaluationDetail{
				{ScoreRange: "0.9-1.0", SampleAnswers: []string{"ans1"}, Explanation: "exp1"}, // This range is not in validScoreRanges
				validScoreEvals[1],
			}),
			true, "score range '0.9-1.0' in score evaluations not found in defined score ranges",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.eval.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("QuizEvaluation.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errText != "" {
				validationErr, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("QuizEvaluation.Validate() error type = %T, want *ValidationError for specific text check", err)
				} else if validationErr.message != tt.errText {
					t.Errorf("QuizEvaluation.Validate() error text = '%s', want specific text '%s'", validationErr.message, tt.errText)
				}
			}
		})
	}
}

func TestNewQuizEvaluation(t *testing.T) {
	quizID := "testQuizID"
	minKeywords := 2
	requiredTopics := []string{"Go", "Testing"}
	scoreRanges := []string{"0-0.4", "0.5-1.0"}
	sampleAnswers := []string{"This is a sample answer."}
	rubricDetails := "Detailed rubric here."
	scoreEvals := []ScoreEvaluationDetail{
		{ScoreRange: "0-0.4", SampleAnswers: []string{"Bad answer"}, Explanation: "Poor explanation"},
		{ScoreRange: "0.5-1.0", SampleAnswers: []string{"Good answer"}, Explanation: "Good explanation"},
	}

	eval := NewQuizEvaluation(quizID, minKeywords, requiredTopics, scoreRanges, sampleAnswers, rubricDetails, scoreEvals)

	if eval.QuizID != quizID {
		t.Errorf("NewQuizEvaluation() QuizID = %s, want %s", eval.QuizID, quizID)
	}
	if eval.MinimumKeywords != minKeywords {
		t.Errorf("NewQuizEvaluation() MinimumKeywords = %d, want %d", eval.MinimumKeywords, minKeywords)
	}
	if len(eval.RequiredTopics) != len(requiredTopics) || eval.RequiredTopics[0] != requiredTopics[0] {
		t.Errorf("NewQuizEvaluation() RequiredTopics = %v, want %v", eval.RequiredTopics, requiredTopics)
	}
	if len(eval.ScoreRanges) != len(scoreRanges) || eval.ScoreRanges[0] != scoreRanges[0] {
		t.Errorf("NewQuizEvaluation() ScoreRanges = %v, want %v", eval.ScoreRanges, scoreRanges)
	}
	if len(eval.SampleAnswers) != len(sampleAnswers) || eval.SampleAnswers[0] != sampleAnswers[0] {
		t.Errorf("NewQuizEvaluation() SampleAnswers = %v, want %v", eval.SampleAnswers, sampleAnswers)
	}
	if eval.RubricDetails != rubricDetails {
		t.Errorf("NewQuizEvaluation() RubricDetails = %s, want %s", eval.RubricDetails, rubricDetails)
	}
	if len(eval.ScoreEvaluations) != len(scoreEvals) {
		t.Errorf("NewQuizEvaluation() len(ScoreEvaluations) = %d, want %d", len(eval.ScoreEvaluations), len(scoreEvals))
	} else if eval.ScoreEvaluations[0].Explanation != scoreEvals[0].Explanation {
		t.Errorf("NewQuizEvaluation() ScoreEvaluations[0].Explanation = %s, want %s", eval.ScoreEvaluations[0].Explanation, scoreEvals[0].Explanation)
	}

	if eval.CreatedAt.IsZero() {
		t.Errorf("NewQuizEvaluation() CreatedAt is zero, want non-zero")
	}
	if eval.UpdatedAt.IsZero() {
		t.Errorf("NewQuizEvaluation() UpdatedAt is zero, want non-zero")
	}
	// ID is set by the constructor using util.NewULID, so it should not be empty.
	// However, testing its exact value is not practical. Just check if it's set.
	// The instruction says "ID will be set by SaveQuiz or here",
	// but NewQuizEvaluation in domain/quiz.go already sets it.
	// Let's assume the intention is to check if it's non-empty if set by constructor.
	// The provided NewQuizEvaluation does NOT set the ID. It's set by repository typically.
	// The instruction for domain/quiz.go's NewQuizEvaluation was:
	// `evaluation.ID = util.NewULID()` IF `evaluation.ID == ""`
	// The constructor for QuizEvaluation in `domain/quiz.go` does NOT set the ID.
	// It's set in `repository/quiz_database_adapter.go SaveQuizEvaluation`
	// So, for `NewQuizEvaluation` unit test, ID should be empty.
	if eval.ID != "" {
		t.Errorf("NewQuizEvaluation() ID = %s, want empty string as it's set by repository", eval.ID)
	}
}
