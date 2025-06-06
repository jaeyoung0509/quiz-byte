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
	categorySeedFilePath = "configs/seed_data/category.json"
	quizSeedFilePath     = "configs/seed_data/initial_english_quizzes.json"
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

	// Step 1: Create basic categories from category.json
	log.Info("Step 1: Creating basic categories", zap.String("path", categorySeedFilePath))
	if err := seedBasicCategories(ctx, db, log); err != nil {
		log.Fatal("Failed to seed basic categories", zap.Error(err))
	}

	// Step 2: Create subcategories and quizzes from initial_english_quizzes.json
	log.Info("Step 2: Creating subcategories and quizzes", zap.String("path", quizSeedFilePath))
	if err := seedQuizData(ctx, db, log); err != nil {
		log.Fatal("Failed to seed quiz data", zap.Error(err))
	}

	log.Info("Initial data seeding process completed.")
}

func seedBasicCategories(ctx context.Context, db *sqlx.DB, log *zap.Logger) error {
	log.Info("Loading basic categories from file", zap.String("path", categorySeedFilePath))
	byteValue, err := os.ReadFile(categorySeedFilePath)
	if err != nil {
		return fmt.Errorf("failed to read category file: %w", err)
	}

	var categoryNames []string
	if err := json.Unmarshal(byteValue, &categoryNames); err != nil {
		return fmt.Errorf("failed to unmarshal category data: %w", err)
	}
	log.Info("Successfully loaded basic categories", zap.Int("categories_count", len(categoryNames)))

	categoryRepo := repository.NewCategoryDatabaseAdapter(db)

	for _, categoryName := range categoryNames {
		log.Info("Processing basic category", zap.String("name", categoryName))

		// Check if category already exists
		existingCategory, err := categoryRepo.GetByName(ctx, categoryName)
		if err != nil {
			return fmt.Errorf("error checking category %s: %w", categoryName, err)
		}

		if existingCategory == nil {
			log.Info("Category not found, creating", zap.String("name", categoryName))
			newCategory := domain.NewCategory(categoryName, "")
			if err := categoryRepo.SaveCategory(ctx, newCategory); err != nil {
				return fmt.Errorf("failed to save category %s: %w", categoryName, err)
			}
			log.Info("Created basic category", zap.String("id", newCategory.ID), zap.String("name", newCategory.Name))
		} else {
			log.Info("Category already exists", zap.String("id", existingCategory.ID), zap.String("name", existingCategory.Name))
		}
	}

	return nil
}

func seedQuizData(ctx context.Context, db *sqlx.DB, log *zap.Logger) error {
	log.Info("Loading quiz data from file", zap.String("path", quizSeedFilePath))
	byteValue, err := os.ReadFile(quizSeedFilePath)
	if err != nil {
		return fmt.Errorf("failed to read quiz file: %w", err)
	}

	var seedCategories []seedmodels.SeedCategory
	if err := json.Unmarshal(byteValue, &seedCategories); err != nil {
		return fmt.Errorf("failed to unmarshal quiz data: %w", err)
	}
	log.Info("Successfully loaded quiz data", zap.Int("categories_count", len(seedCategories)))

	for _, sc := range seedCategories {
		err := seedCategoryData(ctx, db, log, sc)
		if err != nil {
			log.Error("Error seeding category data, transaction rolled back", zap.String("category", sc.Name), zap.Error(err))
			// Continue with other categories instead of stopping
		}
	}

	return nil
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

	// Check if category exists (it should exist from step 1)
	dbCategory, err := txCategoryRepo.GetByName(ctx, seedCat.Name)
	if err != nil {
		return fmt.Errorf("error checking category %s: %w", seedCat.Name, err) // Propagate error to trigger rollback
	}
	if dbCategory == nil {
		return fmt.Errorf("category %s not found - should have been created in step 1", seedCat.Name)
	}
	log.Info("Found existing category.", zap.String("id", dbCategory.ID), zap.String("name", dbCategory.Name))

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
