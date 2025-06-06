package integration

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Helper function to run the migrate command
func runMigrateCommand(t *testing.T, args ...string) (string, error) {
	if testing.Short() {
		t.Skip("Skipping migrate command test in short mode.")
	}
	require.NotNil(t, cfg, "Global config (cfg) is not loaded for migrate command")

	cmdEnv := os.Environ()
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_LOGGER_ENV=%s", cfg.Logger.Env))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_HOST=%s", cfg.DB.Host))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_PORT=%d", cfg.DB.Port))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_USER=%s", cfg.DB.User))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_PASSWORD=%s", cfg.DB.Password))
	cmdEnv = append(cmdEnv, fmt.Sprintf("APP_DB_DBNAME=%s", cfg.DB.DBName))
	// Migrations path is usually configured within the migrate command or relative to it.
	// If MIGRATE_PATH is needed: cmdEnv = append(cmdEnv, "MIGRATE_PATH=../../database/migrations")

	wd, _ := os.Getwd()
	var migrateCmdPath string
	if strings.HasSuffix(wd, "tests/integration") {
		migrateCmdPath = filepath.Join(wd, "..", "..", "cmd", "migrate", "main.go")
	} else { // Assuming ran from project root
		migrateCmdPath = filepath.Join(wd, "cmd", "migrate", "main.go")
	}

	fullArgs := append([]string{"run", migrateCmdPath}, args...)
	cmd := exec.Command("go", fullArgs...)
	cmd.Env = cmdEnv

	outputBytes, err := cmd.CombinedOutput()
	logOutput := string(outputBytes)
	logInstance.Info("Migrate command output", zap.String("args", strings.Join(args, " ")), zap.String("output", logOutput))

	// Do not require.NoError(t, err) here, as some commands might intentionally result in non-zero exit codes
	// that the test wishes to assert (though for up/down, success is expected).
	return logOutput, err
}

// Helper function to get the current migration status
// Returns latest migration id and count of applied migrations
func getMigrationStatusDB(t *testing.T) (string, int, error) {
	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM gorp_migrations")
	if err != nil {
		// If table doesn't exist (e.g., after clean), return zero state
		if strings.Contains(err.Error(), "ORA-00942") || err == sql.ErrNoRows {
			return "0", 0, nil
		}
		return "", 0, fmt.Errorf("failed to query gorp_migrations count: %w", err)
	}

	if count == 0 {
		return "0", 0, nil
	}

	// Get the latest migration id (assuming migrations are applied in order)
	var latestId string
	err = db.Get(&latestId, "SELECT id FROM gorp_migrations ORDER BY applied_at DESC FETCH FIRST 1 ROWS ONLY")
	if err != nil {
		return "", count, fmt.Errorf("failed to query latest migration: %w", err)
	}

	return latestId, count, nil
}

func TestMigrateCommand(t *testing.T) {
	// Initial state: initDatabase in main_test.go already runs migrations up.
	// So, we start by bringing all migrations down.
	t.Log("Running migrate clean to reset...")
	output, err := runMigrateCommand(t, "clean")
	require.NoError(t, err, "migrate clean failed. Output: %s", output)
	assert.Contains(t, output, "Cleaned all tables", "Expected success message for clean")

	latestId, count, err := getMigrationStatusDB(t)
	require.NoError(t, err, "Failed to get migration status after clean")
	assert.Equal(t, 0, count, "Migration count should be 0 after clean")
	assert.Equal(t, "0", latestId, "Latest migration id should be 0 after clean")

	// Test `up`
	t.Log("Running migrate up...")
	output, err = runMigrateCommand(t, "up")
	require.NoError(t, err, "migrate up failed. Output: %s", output)
	assert.Contains(t, output, "Applied", "Expected success message for up")

	latestIdAfterUp, countAfterUp, err := getMigrationStatusDB(t)
	require.NoError(t, err)
	assert.True(t, countAfterUp > 0, "Migration count should be > 0 after up. Got: %d", countAfterUp)
	assert.True(t, latestIdAfterUp != "0", "Latest migration id should be > 0 after first up. Got: %s", latestIdAfterUp)
	initialCount := countAfterUp

	// Test `down` (single step)
	t.Log("Running migrate down (single step)...")
	output, err = runMigrateCommand(t, "down")
	require.NoError(t, err, "migrate down (single) failed. Output: %s", output)
	assert.Contains(t, output, "Rolled back", "Expected success message for single down")

	_, countAfterDown, err := getMigrationStatusDB(t)
	require.NoError(t, err)
	assert.True(t, countAfterDown < initialCount, "Migration count should decrease after down. Initial: %d, After: %d", initialCount, countAfterDown)

	// Test `up` again (to reach latest)
	t.Log("Running migrate up again...")
	output, err = runMigrateCommand(t, "up")
	require.NoError(t, err, "migrate up (second time) failed. Output: %s", output)
	assert.Contains(t, output, "Applied", "Expected success message for second up")

	_, finalCount, err := getMigrationStatusDB(t)
	require.NoError(t, err)
	assert.Equal(t, initialCount, finalCount, "Migration count should be back to the initial count after up again. Initial: %d, Final: %d", initialCount, finalCount)

	// Optional: Verify schema changes (light check)
	// Example: Check if 'users' table exists after full 'up'
	var tableName string
	err = db.Get(&tableName, "SELECT table_name FROM user_tables WHERE table_name = 'USERS'")
	if err == sql.ErrNoRows {
		// For some DBs like PostgreSQL, table names are lowercase unless quoted.
		// Oracle typically uses uppercase.
		err = db.Get(&tableName, "SELECT table_name FROM user_tables WHERE table_name = 'users'")
	}
	require.NoError(t, err, "'USERS' table should exist after migrations are up")
	assert.Equal(t, "USERS", strings.ToUpper(tableName), "Expected 'USERS' table to be found")
}
