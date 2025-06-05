package repository

import (
	"encoding/json"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"testing"
	"time"
	// "github.com/stretchr/testify/assert" // Example, not used to keep it simple
)

func TestToModelQuizEvaluationAndBack(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for consistent comparison
	domainEval := &domain.QuizEvaluation{
		ID:            "eval1",
		QuizID:        "quiz1",
		MinimumKeywords: 2,
		RequiredTopics: []string{"Go", "Structs"},
		ScoreRanges:    []string{"0-0.5", "0.5-1.0"},
		SampleAnswers:  []string{"Sample Ans 1"},
		RubricDetails:  "Some rubric details",
		CreatedAt:      now,
		UpdatedAt:      now,
		ScoreEvaluations: []domain.ScoreEvaluationDetail{
			{ScoreRange: "0-0.5", SampleAnswers: []string{"Bad answer"}, Explanation: "This was not good."},
			{ScoreRange: "0.5-1.0", SampleAnswers: []string{"Good answer!"}, Explanation: "Excellent work!"},
		},
	}

	// 1. Test toModelQuizEvaluation
	modelEval, err := toModelQuizEvaluation(domainEval)
	if err != nil {
		t.Fatalf("toModelQuizEvaluation() error = %v", err)
	}
	if modelEval == nil {
		t.Fatalf("toModelQuizEvaluation() returned nil model")
	}

	// Check basic fields
	if modelEval.ID != domainEval.ID {
		t.Errorf("modelEval.ID = %s, want %s", modelEval.ID, domainEval.ID)
	}
	if modelEval.QuizID != domainEval.QuizID {
		t.Errorf("modelEval.QuizID = %s, want %s", modelEval.QuizID, domainEval.QuizID)
	}
	if modelEval.MinimumKeywords != domainEval.MinimumKeywords {
		t.Errorf("modelEval.MinimumKeywords = %d, want %d", modelEval.MinimumKeywords, domainEval.MinimumKeywords)
	}
	if len(modelEval.RequiredTopics) != len(domainEval.RequiredTopics) || modelEval.RequiredTopics[0] != domainEval.RequiredTopics[0] {
		t.Errorf("modelEval.RequiredTopics = %v, want %v", modelEval.RequiredTopics, domainEval.RequiredTopics)
	}
	if len(modelEval.ScoreRanges) != len(domainEval.ScoreRanges) || modelEval.ScoreRanges[0] != domainEval.ScoreRanges[0] {
		t.Errorf("modelEval.ScoreRanges = %v, want %v", modelEval.ScoreRanges, domainEval.ScoreRanges)
	}
	if len(modelEval.SampleAnswers) != len(domainEval.SampleAnswers) || modelEval.SampleAnswers[0] != domainEval.SampleAnswers[0] {
		t.Errorf("modelEval.SampleAnswers = %v, want %v", modelEval.SampleAnswers, domainEval.SampleAnswers)
	}
	if modelEval.RubricDetails != domainEval.RubricDetails {
		t.Errorf("modelEval.RubricDetails = %s, want %s", modelEval.RubricDetails, domainEval.RubricDetails)
	}
	if !modelEval.CreatedAt.Equal(domainEval.CreatedAt) {
		t.Errorf("modelEval.CreatedAt = %v, want %v", modelEval.CreatedAt, domainEval.CreatedAt)
	}
	if !modelEval.UpdatedAt.Equal(domainEval.UpdatedAt) {
		t.Errorf("modelEval.UpdatedAt = %v, want %v", modelEval.UpdatedAt, domainEval.UpdatedAt)
	}

	// Check JSON marshaling for ScoreEvaluations
	var unmarshaledDetails []domain.ScoreEvaluationDetail
	if modelEval.ScoreEvaluations == "" {
		t.Fatalf("modelEval.ScoreEvaluations is empty, expected JSON string")
	}
	err = json.Unmarshal([]byte(modelEval.ScoreEvaluations), &unmarshaledDetails)
	if err != nil {
		t.Fatalf("Failed to unmarshal modelEval.ScoreEvaluations: %v. JSON was: %s", err, modelEval.ScoreEvaluations)
	}
	if len(unmarshaledDetails) != len(domainEval.ScoreEvaluations) {
		t.Errorf("Unmarshaled details length = %d, want %d", len(unmarshaledDetails), len(domainEval.ScoreEvaluations))
	}
	if len(unmarshaledDetails) > 0 && len(domainEval.ScoreEvaluations) > 0 && unmarshaledDetails[0].Explanation != domainEval.ScoreEvaluations[0].Explanation {
		t.Errorf("Unmarshaled explanation mismatch. Got %s, want %s", unmarshaledDetails[0].Explanation, domainEval.ScoreEvaluations[0].Explanation)
	}

	// 2. Test toDomainQuizEvaluation
	// We need a models.QuizEvaluation, we can use the one we just created (modelEval)
	modelEval.CreatedAt = domainEval.CreatedAt // Ensure time fields are consistent for the round trip
	modelEval.UpdatedAt = domainEval.UpdatedAt

	convertedDomainEval, err := toDomainQuizEvaluation(modelEval)
	if err != nil {
		t.Fatalf("toDomainQuizEvaluation() error = %v", err)
	}
	if convertedDomainEval == nil {
		t.Fatalf("toDomainQuizEvaluation() returned nil")
	}

	if convertedDomainEval.ID != domainEval.ID {
		t.Errorf("Roundtrip ID mismatch: got %s, want %s", convertedDomainEval.ID, domainEval.ID)
	}
	if convertedDomainEval.QuizID != domainEval.QuizID {
		t.Errorf("Roundtrip QuizID mismatch: got %s, want %s", convertedDomainEval.QuizID, domainEval.QuizID)
	}
	if convertedDomainEval.MinimumKeywords != domainEval.MinimumKeywords {
		t.Errorf("Roundtrip MinimumKeywords mismatch: got %d, want %d", convertedDomainEval.MinimumKeywords, domainEval.MinimumKeywords)
	}
	// ... compare other scalar fields ...
	if len(convertedDomainEval.ScoreEvaluations) != len(domainEval.ScoreEvaluations) {
		t.Fatalf("Roundtrip ScoreEvaluations length mismatch. Got %d, want %d", len(convertedDomainEval.ScoreEvaluations), len(domainEval.ScoreEvaluations))
	}
	if len(convertedDomainEval.ScoreEvaluations) > 0 && len(domainEval.ScoreEvaluations) > 0 && convertedDomainEval.ScoreEvaluations[0].Explanation != domainEval.ScoreEvaluations[0].Explanation {
		t.Errorf("Roundtrip ScoreEvaluations[0].Explanation mismatch. Got %s, want %s", convertedDomainEval.ScoreEvaluations[0].Explanation, domainEval.ScoreEvaluations[0].Explanation)
	}
	if !convertedDomainEval.CreatedAt.Equal(domainEval.CreatedAt) {
		t.Errorf("Roundtrip CreatedAt mismatch: got %v, want %v", convertedDomainEval.CreatedAt, domainEval.CreatedAt)
	}
	if !convertedDomainEval.UpdatedAt.Equal(domainEval.UpdatedAt) {
		t.Errorf("Roundtrip UpdatedAt mismatch: got %v, want %v", convertedDomainEval.UpdatedAt, domainEval.UpdatedAt)
	}
}

func TestToModelQuizEvaluation_NilInput(t *testing.T) {
	modelEval, err := toModelQuizEvaluation(nil)
	if err != nil {
		t.Errorf("toModelQuizEvaluation(nil) error = %v, want nil", err)
	}
	if modelEval != nil {
		t.Errorf("toModelQuizEvaluation(nil) = %v, want nil", modelEval)
	}
}

func TestToDomainQuizEvaluation_NilInput(t *testing.T) {
	domainEval, err := toDomainQuizEvaluation(nil)
	if err != nil {
		t.Errorf("toDomainQuizEvaluation(nil) error = %v, want nil", err)
	}
	if domainEval != nil {
		t.Errorf("toDomainQuizEvaluation(nil) = %v, want nil", domainEval)
	}
}

func TestToDomainQuizEvaluation_EmptyScoreEvaluationsJSON(t *testing.T) {
	modelEval := &models.QuizEvaluation{
		ID:               "eval1",
		QuizID:           "quiz1",
		ScoreEvaluations: "", // Empty JSON string
	}
	domainEval, err := toDomainQuizEvaluation(modelEval)
	if err != nil {
		t.Fatalf("toDomainQuizEvaluation with empty JSON error = %v", err)
	}
	if domainEval == nil {
		t.Fatalf("toDomainQuizEvaluation with empty JSON returned nil domainEval")
	}
	if len(domainEval.ScoreEvaluations) != 0 {
		t.Errorf("Expected empty ScoreEvaluations slice, got %d elements", len(domainEval.ScoreEvaluations))
	}
}

func TestToDomainQuizEvaluation_MalformedScoreEvaluationsJSON(t *testing.T) {
	modelEval := &models.QuizEvaluation{
		ID:               "eval1",
		QuizID:           "quiz1",
		ScoreEvaluations: "{not_a_valid_json",
	}
	_, err := toDomainQuizEvaluation(modelEval)
	if err == nil {
		t.Fatalf("toDomainQuizEvaluation with malformed JSON expected error, got nil")
	}
}
