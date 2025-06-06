package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"quiz-byte/cmd/seed_initial_data/internal/seedmodels"
	"quiz-byte/internal/config"
	"quiz-byte/internal/database"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/logger"
	"quiz-byte/internal/repository"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

const (
	seedFilePath = "configs/seed_data/initial_english_quizzes.json"
)

func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func main() {
	ctx := context.Background()
	cfg, err := config.LoadConfig()
	if err != nil {
		// If logger is not initialized yet, use fmt
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	// Assuming cfg.Logger is the correct field for logger configuration
	if err := logger.Initialize(cfg.Logger); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() // Ensure logs are flushed
	log := logger.Get()

	log.Info("Starting initial data seeding process...")
	db, err := database.NewSQLXOracleDB(cfg.GetDSN())
	if err != nil {
		log.Fatal("Failed to connect to Oracle database", zap.Error(err))
	}
	defer db.Close()
	log.Info("Successfully connected to Oracle database.")

	log.Info("Loading seed data from file", zap.String("path", seedFilePath))
	byteValue, err := os.ReadFile(seedFilePath)
	if err != nil {
		log.Fatal("Failed to read seed file", zap.String("path", seedFilePath), zap.Error(err))
	}

	var seedCategories []seedmodels.SeedCategory
	if err := json.Unmarshal(byteValue, &seedCategories); err != nil {
		log.Fatal("Failed to unmarshal seed data", zap.Error(err))
	}
	log.Info("Successfully unmarshalled seed data", zap.Int("categories_loaded", len(seedCategories)))

	for _, sc := range seedCategories {
		err := seedCategoryData(ctx, db, log, sc)
		if err != nil {
			log.Error("Error seeding category, transaction rolled back", zap.String("category", sc.Name), zap.Error(err))
			// Decide if you want to stop on first error or continue with other categories
		}
	}
	log.Info("Initial data seeding process completed.")
}

func seedCategoryData(
	ctx context.Context,
	db *sqlx.DB, // The seeder uses the main DB connection to start a transaction
	log *zap.Logger,
	seedCat seedmodels.SeedCategory,
) (err error) { // Named return for clarity in defer
	log.Info("Processing category", zap.String("name", seedCat.Name))
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for category %s: %w", seedCat.Name, err)
	}
	// Defer a function to handle transaction rollback or commit
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // Attempt to rollback on panic
			panic(p)          // Re-panic after rollback attempt
		} else if err != nil {
			log.Error("Rolling back transaction due to error", zap.Error(err), zap.String("category_name_rb", seedCat.Name))
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Error("Failed to rollback transaction", zap.Error(rbErr))
				// Optionally, you could wrap rbErr into the original err: err = fmt.Errorf("%w; additionally, rollback failed: %v", err, rbErr)
			}
		} else {
			// Commit the transaction if no error occurred
			if cErr := tx.Commit(); cErr != nil {
				log.Error("Failed to commit transaction", zap.Error(cErr))
				err = cErr // Set err so the calling function knows about the commit failure
			} else {
				log.Info("Successfully committed transaction for category", zap.String("name", seedCat.Name))
			}
		}
	}()

	// Repositories are created with the transaction object (tx which implements DBTX)
	txCategoryRepo := repository.NewCategoryDatabaseAdapter(tx)
	txQuizRepo := repository.NewQuizDatabaseAdapter(tx)

	// Check if category exists
	dbCategory, err := txCategoryRepo.GetByName(ctx, seedCat.Name)
	if err != nil {
		return fmt.Errorf("error checking category %s: %w", seedCat.Name, err) // Propagate error to trigger rollback
	}
	if dbCategory == nil {
		log.Info("Category not found, creating.", zap.String("name", seedCat.Name))
		dbCategory = domain.NewCategory(seedCat.Name, seedCat.Description)
		if err = txCategoryRepo.SaveCategory(ctx, dbCategory); err != nil {
			return fmt.Errorf("failed to save category %s: %w", seedCat.Name, err) // Propagate error
		}
		log.Info("Created category.", zap.String("id", dbCategory.ID), zap.String("name", dbCategory.Name))
	} else {
		log.Info("Category exists.", zap.String("id", dbCategory.ID), zap.String("name", dbCategory.Name))
	}

	for _, seedSubCat := range seedCat.SubCategories {
		log.Info("Processing sub-category", zap.String("name", seedSubCat.Name), zap.String("parent_category", dbCategory.Name))
		dbSubCategory, errSub := txCategoryRepo.GetByNameAndCategoryID(ctx, seedSubCat.Name, dbCategory.ID)
		if errSub != nil {
			return fmt.Errorf("error checking sub-category %s: %w", seedSubCat.Name, errSub) // Propagate error
		}
		if dbSubCategory == nil {
			log.Info("Sub-category not found, creating.", zap.String("name", seedSubCat.Name))
			dbSubCategory = domain.NewSubCategory(dbCategory.ID, seedSubCat.Name, seedSubCat.Description)
			if errSub = txCategoryRepo.SaveSubCategory(ctx, dbSubCategory); errSub != nil {
				return fmt.Errorf("failed to save sub-category %s: %w", seedSubCat.Name, errSub) // Propagate error
			}
			log.Info("Created sub-category.", zap.String("id", dbSubCategory.ID), zap.String("name", dbSubCategory.Name))
		} else {
			log.Info("Sub-category exists.", zap.String("id", dbSubCategory.ID), zap.String("name", dbSubCategory.Name))
		}

		for _, seedQuiz := range seedSubCat.Quizzes {
			log.Info("Processing quiz", zap.String("question_preview", firstN(seedQuiz.Question, 20)), zap.String("parent_sub_category", dbSubCategory.Name))
			difficultyInt := domain.DifficultyToInt(seedQuiz.Difficulty)
			// Assuming NewQuiz returns *domain.Quiz
			domainQuiz := domain.NewQuiz(seedQuiz.Question, seedQuiz.ModelAnswers, seedQuiz.Keywords, difficultyInt, dbSubCategory.ID)
			if errQ := txQuizRepo.SaveQuiz(ctx, domainQuiz); errQ != nil {
				return fmt.Errorf("failed to save quiz '%s': %w", firstN(seedQuiz.Question, 50), errQ) // Propagate error
			}
			log.Info("Successfully created quiz.", zap.String("id", domainQuiz.ID)) // ID should be populated by SaveQuiz
		}
	}
	return nil // Mark success for commit
}
