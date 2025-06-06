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

// Helper function to get the current migration version and dirty status
// Returns version as string, dirty as bool, and error
func getMigrationVersionDB(t *testing.T) (string, bool, error) {
	var version sql.NullInt64 // Use sql.NullInt64 for version that can be NULL
	var dirty sql.NullBool    // Use sql.NullBool for dirty status

	// In Oracle, schema_migrations table name might be case-sensitive if created with quotes.
	// Assuming it's uppercase as is common.
	// The query might differ slightly based on exact schema_migrations table structure used by the migrate library.
	// This is a common structure for github.com/golang-migrate/migrate.
	err := db.QueryRow("SELECT version, dirty FROM schema_migrations FETCH FIRST 1 ROWS ONLY").Scan(&version, &dirty)
	if err != nil {
		if err == sql.ErrNoRows {
			// No rows typically means version 0, no migrations applied, or table missing (after full down)
			return "0", false, nil
		}
		return "", false, fmt.Errorf("failed to query schema_migrations: %w", err)
	}

	if !version.Valid { // Should not happen if a migration ran, but handle defensively
		return "0", dirty.Bool, nil // Treat NULL version as 0
	}

	return fmt.Sprintf("%d", version.Int64), dirty.Bool, nil
}

func TestMigrateCommand(t *testing.T) {
	// Initial state: initDatabase in main_test.go already runs migrations up.
	// So, we start by bringing all migrations down.
	t.Log("Running migrate down --all to reset...")
	output, err := runMigrateCommand(t, "down", "--all") // Assuming --all is supported by the custom migrate cmd
	require.NoError(t, err, "migrate down --all failed. Output: %s", output)
	assert.Contains(t, output, "Successfully rolled back all migrations", "Expected success message for down --all")

	currentVersion, dirty, err := getMigrationVersionDB(t)
	require.NoError(t, err, "Failed to get migration version after down --all")
	assert.False(t, dirty, "DB should not be dirty after down --all")
	// After full down, version table might be empty or version 0.
	// If table is dropped and recreated by migrate, ErrNoRows is possible, handled by getMigrationVersionDB.
	assert.Equal(t, "0", currentVersion, "Version should be 0 after down --all")

	// Test `up`
	t.Log("Running migrate up...")
	output, err = runMigrateCommand(t, "up")
	require.NoError(t, err, "migrate up failed. Output: %s", output)
	assert.Contains(t, output, "Migrations applied successfully!", "Expected success message for up")

	latestVersion, dirty, err := getMigrationVersionDB(t)
	require.NoError(t, err)
	assert.False(t, dirty, "DB should not be dirty after up")
	// We need to know the "latest known migration version". This is hard to get dynamically
	// without parsing migration files. For now, assert it's > 0.
	// A more robust test would check against a specific expected version.
	assert.True(t, latestVersion != "0", "Version should be > 0 after first up. Got: %s", latestVersion)
	initialLatestVersion := latestVersion // Store for later comparison

	// Test `down` (single step)
	t.Log("Running migrate down (single step)...")
	output, err = runMigrateCommand(t, "down")
	require.NoError(t, err, "migrate down (single) failed. Output: %s", output)
	// Message might be "Successfully rolled back 1 migration(s)"
	assert.Contains(t, output, "Successfully rolled back", "Expected success message for single down")

	versionAfterSingleDown, dirty, err := getMigrationVersionDB(t)
	require.NoError(t, err)
	assert.False(t, dirty, "DB should not be dirty after single down")
	assert.NotEqual(t, initialLatestVersion, versionAfterSingleDown, "Version should have changed after single down")
	// Simple check: version should be less. This assumes numeric, sequential versions.
	// This might not hold if versions are timestamp-based and not perfectly sequential numbers.
	// For now, we'll rely on NotEqual. A better check would be to parse file names.

	// Test `up` again (to reach latest)
	t.Log("Running migrate up again...")
	output, err = runMigrateCommand(t, "up")
	require.NoError(t, err, "migrate up (second time) failed. Output: %s", output)
	assert.Contains(t, output, "Migrations applied successfully!", "Expected success message for second up")

	finalVersion, dirty, err := getMigrationVersionDB(t)
	require.NoError(t, err)
	assert.False(t, dirty, "DB should not be dirty after second up")
	assert.Equal(t, initialLatestVersion, finalVersion, "Version should be back to the initial latest version after up again")

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
