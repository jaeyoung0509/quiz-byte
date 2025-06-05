package service

import (
	"context" // Keep: used by test functions
	"errors"
	"testing"

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	// "quiz-byte/internal/util" // For actual CosineSimilarity - util.CosineSimilarity is not directly called in test
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// --- Test Suite ---
func TestGenerateNewQuizzesAndSave_Success_NewUniqueQuizzes(t *testing.T) {
	mockQuizRepo := new(MockQuizRepository)
	mockCategoryRepo := new(MockCategoryRepository) // Not used in current GenerateNewQuizzesAndSave
	mockEmbeddingService := new(MockEmbeddingService)
	mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{SimilarityThreshold: 0.9},
		Batch:     config.BatchConfig{NumQuestionsPerSubCategory: 1},
	}

	batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument

	ctx := context.Background()
	subCategoryID1 := "subCat1"

	// Mock expectations
	mockQuizRepo.On("GetAllSubCategories", ctx).Return([]string{subCategoryID1}, nil).Once()
	mockQuizRepo.On("GetQuizzesBySubCategory", ctx, subCategoryID1).Return([]*domain.Quiz{}, nil).Once() // No existing quizzes

	generatedQuiz1 := &domain.NewQuizData{
		Question:    "New Q1?",
		ModelAnswer: "Ans1",
		Keywords:    []string{"k1", "k2"},
		Difficulty:  "easy",
	}
	mockQuizGenSvc.On("GenerateQuizCandidates", ctx, subCategoryID1, mock.MatchedBy(func(arg []string) bool { return arg == nil }), 1).Return([]*domain.NewQuizData{generatedQuiz1}, nil).Once()

	embeddingForGeneratedQ1 := []float32{0.1, 0.2, 0.3}
	mockEmbeddingService.On("Generate", ctx, generatedQuiz1.Question).Return(embeddingForGeneratedQ1, nil).Once()

	// Expect SaveQuiz to be called because it's unique (no existing quizzes)
	mockQuizRepo.On("SaveQuiz", ctx, mock.MatchedBy(func(quiz *domain.Quiz) bool {
		return quiz.Question == generatedQuiz1.Question && quiz.SubCategoryID == subCategoryID1
	})).Return(nil).Once()

	// Execute
	err := batchSvc.GenerateNewQuizzesAndSave(ctx)

	// Assertions
	assert.NoError(t, err)
	mockQuizRepo.AssertExpectations(t)
	mockQuizGenSvc.AssertExpectations(t) // Updated mock
	mockEmbeddingService.AssertExpectations(t)
	mockQuizRepo.AssertCalled(t, "SaveQuiz", ctx, mock.AnythingOfType("*domain.Quiz"))
}

func TestGenerateNewQuizzesAndSave_Success_SomeSimilarQuizzes(t *testing.T) {
	mockQuizRepo := new(MockQuizRepository)
	mockCategoryRepo := new(MockCategoryRepository)
	mockEmbeddingService := new(MockEmbeddingService)
	mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{SimilarityThreshold: 0.95}, // Higher threshold
		Batch:     config.BatchConfig{NumQuestionsPerSubCategory: 2},     // Generate 2 questions
	}

	batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
	ctx := context.Background()
	subCategoryID1 := "subCatSimilar"

	existingQuiz1 := &domain.Quiz{
		ID:            "existing1",
		Question:      "Existing Question 1",
		ModelAnswers:  []string{"Existing Answer 1"},
		Keywords:      []string{"exist", "q1"},
		Difficulty:    1,
		SubCategoryID: subCategoryID1,
	}
	mockQuizRepo.On("GetAllSubCategories", ctx).Return([]string{subCategoryID1}, nil).Once()
	mockQuizRepo.On("GetQuizzesBySubCategory", ctx, subCategoryID1).Return([]*domain.Quiz{existingQuiz1}, nil).Once()

	// LLM generates two quizzes
	generatedQuizUnique := &domain.NewQuizData{
		Question:    "Totally New Unique Question",
		ModelAnswer: "Unique Ans",
		Keywords:    []string{"unique", "new"},
		Difficulty:  "medium",
	}
	generatedQuizSimilar := &domain.NewQuizData{ // This one will be similar to existingQuiz1
		Question:    "Very Similar Existing Question 1",
		ModelAnswer: "Similar Ans",
		Keywords:    []string{"exist", "q1", "similar"},
		Difficulty:  "easy",
	}
	mockQuizGenSvc.On("GenerateQuizCandidates", ctx, subCategoryID1, mock.AnythingOfType("[]string"), 2).Return([]*domain.NewQuizData{generatedQuizUnique, generatedQuizSimilar}, nil).Once() // Updated mock call

	// Embeddings
	embeddingExistingQ1 := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	embeddingGeneratedUnique := []float32{0.5, 0.4, 0.3, 0.2, 0.1} // Different
	embeddingGeneratedSimilar := []float32{0.1, 0.2, 0.3, 0.4, 0.5} // Same as existingQ1 for test

	// Order of Generate calls can be tricky if not strictly sequential in code.
	// For existing quiz (might be cached or generated)
	mockEmbeddingService.On("Generate", ctx, existingQuiz1.Question).Return(embeddingExistingQ1, nil).Maybe() // Maybe, because it could be cached

	// For generated unique quiz
	mockEmbeddingService.On("Generate", ctx, generatedQuizUnique.Question).Return(embeddingGeneratedUnique, nil).Once()
	// For generated similar quiz
	mockEmbeddingService.On("Generate", ctx, generatedQuizSimilar.Question).Return(embeddingGeneratedSimilar, nil).Once()

	// CosineSimilarity will be called. We are using the real util.CosineSimilarity.
	// If we were mocking it:
	// mockUtil.On("CosineSimilarity", embeddingGeneratedUnique, embeddingExistingQ1).Return(0.5, nil) // Low similarity
	// mockUtil.On("CosineSimilarity", embeddingGeneratedSimilar, embeddingExistingQ1).Return(0.98, nil) // High similarity

	// Expect SaveQuiz to be called only for the unique one
	mockQuizRepo.On("SaveQuiz", ctx, mock.MatchedBy(func(quiz *domain.Quiz) bool {
		return quiz.Question == generatedQuizUnique.Question
	})).Return(nil).Once()


	err := batchSvc.GenerateNewQuizzesAndSave(ctx)
	assert.NoError(t, err)

	mockQuizRepo.AssertExpectations(t)
	mockQuizGenSvc.AssertExpectations(t) // Updated mock
	mockEmbeddingService.AssertExpectations(t)

	// Assert SaveQuiz was called once (for the unique quiz)
	mockQuizRepo.AssertNumberOfCalls(t, "SaveQuiz", 1)
}

func TestGenerateNewQuizzesAndSave_Error_GetAllSubCategoriesFails(t *testing.T) {
	mockQuizRepo := new(MockQuizRepository)
	// ... other mocks are not strictly necessary for this specific error path but good practice
	mockCategoryRepo := new(MockCategoryRepository)
	mockEmbeddingService := new(MockEmbeddingService)
	mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{} // Config might not be deeply accessed if it fails early

	batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
	ctx := context.Background()

	expectedError := errors.New("db error on GetAllSubCategories")
	mockQuizRepo.On("GetAllSubCategories", ctx).Return(nil, expectedError).Once()

	err := batchSvc.GenerateNewQuizzesAndSave(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), expectedError.Error())
	mockQuizRepo.AssertExpectations(t)
	// Ensure other downstream calls were not made
	mockQuizGenSvc.AssertNotCalled(t, "GenerateQuizCandidates", mock.Anything, mock.Anything, mock.Anything, mock.Anything) // Updated mock
	mockQuizRepo.AssertNotCalled(t, "SaveQuiz", mock.Anything, mock.Anything)
}

func TestGenerateNewQuizzesAndSave_Error_GetQuizzesBySubCategoryFails(t *testing.T) {
    mockQuizRepo := new(MockQuizRepository)
    mockCategoryRepo := new(MockCategoryRepository)
    mockEmbeddingService := new(MockEmbeddingService)
    mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

    logger, _ := zap.NewDevelopment()
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{SimilarityThreshold: 0.9},
        Batch:     config.BatchConfig{NumQuestionsPerSubCategory: 1},
    }
    batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
    ctx := context.Background()
    subCategoryID1 := "subCatErr"
    expectedError := errors.New("db error on GetQuizzesBySubCategory")

    mockQuizRepo.On("GetAllSubCategories", ctx).Return([]string{subCategoryID1}, nil).Once()
    mockQuizRepo.On("GetQuizzesBySubCategory", ctx, subCategoryID1).Return(nil, expectedError).Once()

    // The service currently logs this error and continues to the next subcategory (if any).
    // If it were the only subcategory, the overall function would still return nil.
    // To test this specific error causing a non-nil return, the service logic would need to change,
    // or this test needs to reflect that it might still be a `nil` error overall for the batch.
    // For now, let's assume the batch continues, so overall error is nil.
    err := batchSvc.GenerateNewQuizzesAndSave(ctx)
    assert.NoError(t, err) // Because the service continues on this specific error

    mockQuizRepo.AssertExpectations(t)
    mockQuizGenSvc.AssertNotCalled(t, "GenerateQuizCandidates", mock.Anything, mock.Anything, mock.Anything, mock.Anything) // Updated mock
}


func TestGenerateNewQuizzesAndSave_Error_LLMClientFails(t *testing.T) { // Name will be updated to reflect QuizGenerationService
    mockQuizRepo := new(MockQuizRepository)
    mockCategoryRepo := new(MockCategoryRepository)
    mockEmbeddingService := new(MockEmbeddingService)
    mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

    logger, _ := zap.NewDevelopment()
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{SimilarityThreshold: 0.9},
        Batch:     config.BatchConfig{NumQuestionsPerSubCategory: 1},
    }
    batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
    ctx := context.Background()
    subCategoryID1 := "subCatLLMErr"
    expectedError := errors.New("LLM failed")

    mockQuizRepo.On("GetAllSubCategories", ctx).Return([]string{subCategoryID1}, nil).Once()
    mockQuizRepo.On("GetQuizzesBySubCategory", ctx, subCategoryID1).Return([]*domain.Quiz{}, nil).Once()
	mockQuizGenSvc.On("GenerateQuizCandidates", ctx, subCategoryID1, mock.MatchedBy(func(arg []string) bool { return arg == nil }), 1).Return(nil, expectedError).Once()

    // Service logs and continues
    err := batchSvc.GenerateNewQuizzesAndSave(ctx)
    assert.NoError(t, err) // Batch continues

    mockQuizRepo.AssertExpectations(t)
    mockQuizGenSvc.AssertExpectations(t) // Updated mock
    mockEmbeddingService.AssertNotCalled(t, "Generate", mock.Anything, mock.Anything)
    mockQuizRepo.AssertNotCalled(t, "SaveQuiz", mock.Anything, mock.Anything)
}

func TestGenerateNewQuizzesAndSave_Error_EmbeddingServiceFailsOnNewQuiz(t *testing.T) {
    mockQuizRepo := new(MockQuizRepository)
    mockCategoryRepo := new(MockCategoryRepository)
    mockEmbeddingService := new(MockEmbeddingService)
    mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

    logger, _ := zap.NewDevelopment()
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{SimilarityThreshold: 0.9},
        Batch:     config.BatchConfig{NumQuestionsPerSubCategory: 1},
    }
    batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
    ctx := context.Background()
    subCategoryID1 := "subCatEmbedErr"

    generatedQuiz1 := &domain.NewQuizData{Question: "Q Embed Err", ModelAnswer: "A", Keywords: []string{"k"}, Difficulty: "easy"}
    expectedError := errors.New("embedding failed")

    mockQuizRepo.On("GetAllSubCategories", ctx).Return([]string{subCategoryID1}, nil).Once()
    mockQuizRepo.On("GetQuizzesBySubCategory", ctx, subCategoryID1).Return([]*domain.Quiz{}, nil).Once()
	mockQuizGenSvc.On("GenerateQuizCandidates", ctx, subCategoryID1, mock.MatchedBy(func(arg []string) bool { return arg == nil }), 1).Return([]*domain.NewQuizData{generatedQuiz1}, nil).Once()
    mockEmbeddingService.On("Generate", ctx, generatedQuiz1.Question).Return(nil, expectedError).Once()

    // Service logs and continues
    err := batchSvc.GenerateNewQuizzesAndSave(ctx)
    assert.NoError(t, err) // Batch continues

    mockQuizRepo.AssertExpectations(t)
    mockQuizGenSvc.AssertExpectations(t) // Updated mock
    mockEmbeddingService.AssertExpectations(t)
    mockQuizRepo.AssertNotCalled(t, "SaveQuiz", mock.Anything, mock.Anything)
}


func TestGenerateNewQuizzesAndSave_Error_SaveQuizFails(t *testing.T) {
    mockQuizRepo := new(MockQuizRepository)
    mockCategoryRepo := new(MockCategoryRepository)
    mockEmbeddingService := new(MockEmbeddingService)
    mockQuizGenSvc := new(MockQuizGenerationService) // Renamed from mockLLMClient

    logger, _ := zap.NewDevelopment()
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{SimilarityThreshold: 0.9},
        Batch:     config.BatchConfig{NumQuestionsPerSubCategory: 1},
    }
    batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
    ctx := context.Background()
    subCategoryID1 := "subCatSaveErr"

    generatedQuiz1 := &domain.NewQuizData{Question: "Q Save Err", ModelAnswer: "A", Keywords: []string{"k"}, Difficulty: "easy"}
    embeddingForGeneratedQ1 := []float32{0.1, 0.2, 0.3}
    expectedError := errors.New("save failed")

    mockQuizRepo.On("GetAllSubCategories", ctx).Return([]string{subCategoryID1}, nil).Once()
    mockQuizRepo.On("GetQuizzesBySubCategory", ctx, subCategoryID1).Return([]*domain.Quiz{}, nil).Once()
    mockQuizGenSvc.On("GenerateQuizCandidates", ctx, subCategoryID1, mock.MatchedBy(func(arg []string) bool { return arg == nil }), 1).Return([]*domain.NewQuizData{generatedQuiz1}, nil).Once()
    mockEmbeddingService.On("Generate", ctx, generatedQuiz1.Question).Return(embeddingForGeneratedQ1, nil).Once()
    mockQuizRepo.On("SaveQuiz", ctx, mock.MatchedBy(func(quiz *domain.Quiz) bool {
        return quiz.Question == generatedQuiz1.Question
    })).Return(expectedError).Once()

    // Service logs and continues
    err := batchSvc.GenerateNewQuizzesAndSave(ctx)
    assert.NoError(t, err) // Batch continues

    mockQuizRepo.AssertExpectations(t)
    mockQuizGenSvc.AssertExpectations(t) // Updated mock
    mockEmbeddingService.AssertExpectations(t)
}

// TODO: Add TestGenerateNewQuizzesAndSave_Error_EmbeddingServiceFailsOnExistingQuiz if caching is off or for first load
// This test would be similar to FailsOnNewQuiz but the error would come from embedding an existing quiz.
// The current service logic might make this hard to distinguish in its effect if it simply continues.

// Note on util.CosineSimilarity:
// It's directly used. If it were an interface, it could be mocked.
// For testing specific similarity values, you can control the embeddings returned by MockEmbeddingService.
// For example, make two embeddings identical to force high similarity, or very different for low similarity.
// This is demonstrated in TestGenerateNewQuizzesAndSave_Success_SomeSimilarQuizzes.

// Note on SubCategoryName for LLM:
// The test currently passes subCategoryID as subCategoryName to the LLM mock.
// If the actual service needed to fetch the name, CategoryRepository would need mocking and usage.
// The current batch_service.go uses subCategoryID directly.
// `subCategoryNameForLLM := subCategoryID`
// This means the QuizGenerationService mock should expect subCategoryID as the subCategoryName argument.
// The tests reflect this by using subCategoryID in `mockQuizGenSvc.On("GenerateQuizCandidates", ctx, subCategoryID1, ...)`

// Helper for creating a basic config for tests
func getTestConfig() *config.Config {
	return &config.Config{
		Embedding: config.EmbeddingConfig{
			SimilarityThreshold: 0.9,
		},
		Batch: config.BatchConfig{
			NumQuestionsPerSubCategory: 2,
		},
		// Add other necessary fields if your service starts using them
	}
}

// Helper for creating a test logger
func getTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment() // Or zap.NewNop() for less output
	return logger
}

// Example of a more structured test table (can be used for some error cases or variations)
/*
func TestGenerateNewQuizzesAndSave_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(qr *MockQuizRepository, cr *MockCategoryRepository, es *MockEmbeddingService, qgs *MockQuizGenerationService) // Updated param type
		expectedError bool
		// ... other fields to assert ...
	}{
		// ... test cases ...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuizRepo := new(MockQuizRepository)
			mockCategoryRepo := new(MockCategoryRepository)
			mockEmbeddingService := new(MockEmbeddingService)
			mockQuizGenSvc := new(MockQuizGenerationService) // Renamed

			logger := getTestLogger()
			cfg := getTestConfig()

			if tt.setupMocks != nil {
				tt.setupMocks(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc) // Updated argument
			}

			batchSvc := NewBatchService(mockQuizRepo, mockCategoryRepo, mockEmbeddingService, mockQuizGenSvc, cfg, logger) // Updated argument
			err := batchSvc.GenerateNewQuizzesAndSave(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			// Add more assertions based on tt.fields
			mockQuizRepo.AssertExpectations(t)
			mockCategoryRepo.AssertExpectations(t)
			mockEmbeddingService.AssertExpectations(t)
			mockQuizGenSvc.AssertExpectations(t) // Updated mock
		})
	}
}
*/

// The actual CosineSimilarity from util is used.
// If there was a desire to mock it, it would require util to expose an interface
// or to use a more advanced mocking technique like monkey patching (generally discouraged).
// Controlling the inputs (embeddings) to the real CosineSimilarity function is the current approach.
// The tests `TestGenerateNewQuizzesAndSave_Success_NewUniqueQuizzes` and
// `TestGenerateNewQuizzesAndSave_Success_SomeSimilarQuizzes` demonstrate this by providing
// specific mock embeddings.

// Ensure all required methods for QuizRepository are present in the mock
var _ domain.QuizRepository = (*MockQuizRepository)(nil)
var _ domain.CategoryRepository = (*MockCategoryRepository)(nil)
var _ domain.EmbeddingService = (*MockEmbeddingService)(nil)
var _ domain.QuizGenerationService = (*MockQuizGenerationService)(nil) // Updated static assertion
