package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBatchAddQuestions_SuccessAndIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping batch command test in short mode.")
	}
	// Ensure cfg is loaded (it's global from main_test.go)
	require.NotNil(t, cfg, "Global config (cfg) is not loaded")

	// Setup: Seed initial categories and subcategories directly into the DB
	catID := util.NewULID()
	catName := "Batch Test Category " + catID
	_, err := db.Exec(`INSERT INTO categories (id, name, description, created_at, updated_at)
						VALUES (:1, :2, :3, :4, :5)`,
		catID, catName, "Category for batch add questions test", time.Now(), time.Now())
	require.NoError(t, err, "Failed to insert test category")

	subCatID := util.NewULID()
	subCatName := "Batch Test SubCategory " + subCatID
	_, err = db.Exec(`INSERT INTO sub_categories (id, category_id, name, description, created_at, updated_at)
						VALUES (:1, :2, :3, :4, :5, :6)`,
		subCatID, catID, subCatName, "SubCategory for batch add questions test", time.Now(), time.Now())
	require.NoError(t, err, "Failed to insert test subcategory")
	logInstance.Info("Seeded category and subcategory for batch test", zap.String("catID", catID), zap.String("subCatID", subCatID))

	// Prepare command environment variables from the loaded cfg
	cmdEnv := os.Environ() // Start with current environment
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_LOGGER_ENV=%s", cfg.Logger.Env))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_HOST=%s", cfg.DB.Host))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_PORT=%d", cfg.DB.Port))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_USER=%s", cfg.DB.User))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_PASSWORD=%s", cfg.DB.Password))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_NAME=%s", cfg.DB.DBName))
	// Add any other necessary env vars, e.g., API keys, even if default/mocked for tests
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_LLM_PROVIDERS_GEMINI_API_KEY=%s", cfg.LLMProviders.Gemini.APIKey)) // Needed by config load
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_EMBEDDING_OPENAI_API_KEY=%s", cfg.Embedding.OpenAI.APIKey))        // Needed by config load
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_EMBEDDING_SOURCE=%s", cfg.Embedding.Source))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_LLM_PROVIDERS_OLLAMA_SERVER_URL=%s", cfg.LLMProviders.OllamaServerURL))

	// Determine the path to the main.go for batch_add_questions
	// Assuming tests are run from the root of the project or tests/integration directory
	// Adjust path as necessary. `filepath.Join` is safer.
	wd, _ := os.Getwd()
	logInstance.Info("Current working directory for test execution", zap.String("wd", wd))

	var batchCmdPath string
	if strings.HasSuffix(wd, "tests/integration") {
		batchCmdPath = filepath.Join(wd, "..", "..", "cmd", "batch_add_questions", "main.go")
	} else { // Assuming ran from project root
		batchCmdPath = filepath.Join(wd, "cmd", "batch_add_questions", "main.go")
	}
	logInstance.Info("Calculated path for batch command", zap.String("path", batchCmdPath))

	// --- Execute Command (First Run) ---
	t.Log("Executing batch_add_questions command (First Run)...")
	cmd := exec.Command("go", "run", batchCmdPath)
	cmd.Env = cmdEnv
	outputBytes, err := cmd.CombinedOutput()
	logOutput := string(outputBytes)
	t.Logf("First run output:\n%s", logOutput)
	require.NoError(t, err, "Batch command failed on first run. Output: %s", logOutput)
	assert.Contains(t, logOutput, "Batch process completed successfully", "Expected success message in output")

	// --- Database Verification (First Run) ---
	var initialQuizCount, initialEvalCount int
	err = db.Get(&initialQuizCount, "SELECT COUNT(*) FROM quizzes WHERE sub_category_id = :1", subCatID)
	require.NoError(t, err)
	// Default NumQuestionsPerSubCategory is 3. This might be configurable.
	// For now, assert > 0. If cfg.Batch.NumQuestionsPerSubCategory is accessible, use it.
	// Let's assume default is 3 for this test.
	expectedQuestionsFirstRun := cfg.Batch.NumQuestionsPerSubCategory
	if expectedQuestionsFirstRun == 0 {
		expectedQuestionsFirstRun = 3
	} // Default fallback if not in config for some reason
	assert.Equal(t, expectedQuestionsFirstRun, initialQuizCount, "Unexpected number of quizzes after first run")

	var quizzes []models.Quiz
	err = db.Select(&quizzes, "SELECT * FROM quizzes WHERE sub_category_id = :1", subCatID)
	require.NoError(t, err)
	require.Len(t, quizzes, expectedQuestionsFirstRun, "Mismatch in fetched quizzes length")

	for _, quiz := range quizzes {
		var evalCount int
		err = db.Get(&evalCount, "SELECT COUNT(*) FROM quiz_evaluations WHERE quiz_id = :1", quiz.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, evalCount, "Expected 1 quiz_evaluation record for quiz_id %s", quiz.ID)

		if evalCount == 1 {
			var evalRecord models.QuizEvaluation
			err = db.Get(&evalRecord, "SELECT score_evaluations FROM quiz_evaluations WHERE quiz_id = :1", quiz.ID)
			require.NoError(t, err)

			var scoreEvals []domain.ScoreEvaluationDetail
			// score_evaluations in DB is TEXT, assuming it's a JSON string
			err = json.Unmarshal([]byte(evalRecord.ScoreEvaluations), &scoreEvals)
			require.NoError(t, err, "Failed to unmarshal score_evaluations JSON for quiz %s. JSON: %s", quiz.ID, evalRecord.ScoreEvaluations)

			// Check structure of simulated data
			assert.Len(t, scoreEvals, 3, "Expected 3 score evaluation details for quiz %s", quiz.ID) // Default has 3 ranges
			for _, detail := range scoreEvals {
				assert.Contains(t, detail.Explanation, "Simulated explanation for score range", "Unexpected explanation format for quiz %s", quiz.ID)
				assert.NotEmpty(t, detail.SampleAnswers, "Sample answers should not be empty for quiz %s, range %s", quiz.ID, detail.ScoreRange)
			}
		}
	}

	err = db.Get(&initialEvalCount, "SELECT COUNT(*) FROM quiz_evaluations qe JOIN quizzes q ON qe.quiz_id = q.id WHERE q.sub_category_id = :1", subCatID)
	require.NoError(t, err)
	assert.Equal(t, expectedQuestionsFirstRun, initialEvalCount, "Number of evaluations should match number of quizzes")

	// --- Execute Command (Second Run - Idempotency Check) ---
	t.Log("Executing batch_add_questions command (Second Run - Idempotency)...")
	cmdSecondRun := exec.Command("go", "run", batchCmdPath)
	cmdSecondRun.Env = cmdEnv
	outputBytesSecond, errSecond := cmdSecondRun.CombinedOutput()
	logOutputSecond := string(outputBytesSecond)
	t.Logf("Second run output:\n%s", logOutputSecond)
	require.NoError(t, errSecond, "Batch command failed on second run. Output: %s", logOutputSecond)
	// The message might indicate that no new quizzes were added if it's truly idempotent and subcategory is "full"
	// For now, just check for successful completion.
	// assert.Contains(t, logOutputSecond, "Batch process completed successfully", "Expected success message in output on second run")
	// Or it might say "No new quizzes were generated for subcategory..." if it's full based on NumQuestionsPerSubCategory.
	// This depends on the exact logic of batch_add_questions.
	// A less brittle check is that it completed without error.

	// --- Database Verification (Second Run) ---
	var finalQuizCount, finalEvalCount int
	err = db.Get(&finalQuizCount, "SELECT COUNT(*) FROM quizzes WHERE sub_category_id = :1", subCatID)
	require.NoError(t, err)
	err = db.Get(&finalEvalCount, "SELECT COUNT(*) FROM quiz_evaluations qe JOIN quizzes q ON qe.quiz_id = q.id WHERE q.sub_category_id = :1", subCatID)
	require.NoError(t, err)

	// Idempotency: If the batch command is designed to fill up to NumQuestionsPerSubCategory
	// and it already did so in the first run, then the counts should be identical.
	assert.Equal(t, initialQuizCount, finalQuizCount, "Quiz count should remain the same after second run if subcategory was filled")
	assert.Equal(t, initialEvalCount, finalEvalCount, "Evaluation count should remain the same after second run if subcategory was filled")

	// Further check: ensure no duplicate questions (by question text)
	var allQuizzesInSubCat []models.Quiz
	err = db.Select(&allQuizzesInSubCat, "SELECT question FROM quizzes WHERE sub_category_id = :1", subCatID)
	require.NoError(t, err)

	questionTexts := make(map[string]int)
	for _, q := range allQuizzesInSubCat {
		questionTexts[q.Question]++
	}
	for text, count := range questionTexts {
		assert.Equal(t, 1, count, "Found duplicate question text after idempotency run: %s", text)
	}
}
