package repository

import (
	"context"
	"database/sql"
	"fmt"
	"quiz-byte/internal/domain" // For potential domain level errors or shared constants
	"quiz-byte/internal/dto"    // For DTOs like AttemptFilters, Pagination
	"quiz-byte/internal/repository/models"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// UserQuizAttemptRepository defines the interface for user quiz attempt data operations.
type UserQuizAttemptRepository interface {
	CreateAttempt(ctx context.Context, attempt *models.UserQuizAttempt) error
	GetAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]models.UserQuizAttempt, int, error)
	GetIncorrectAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]models.UserQuizAttempt, int, error)
}

// sqlxUserQuizAttemptRepository implements UserQuizAttemptRepository using sqlx.
type sqlxUserQuizAttemptRepository struct {
	db *sqlx.DB
}

// NewSQLXUserQuizAttemptRepository creates a new instance of sqlxUserQuizAttemptRepository.
func NewSQLXUserQuizAttemptRepository(db *sqlx.DB) UserQuizAttemptRepository {
	return &sqlxUserQuizAttemptRepository{db: db}
}

// CreateAttempt inserts a new quiz attempt into the database.
func (r *sqlxUserQuizAttemptRepository) CreateAttempt(ctx context.Context, attempt *models.UserQuizAttempt) error {
	query := `INSERT INTO user_quiz_attempts (id, user_id, quiz_id, user_answer, llm_score, llm_explanation, llm_keyword_matches, llm_completeness, llm_relevance, llm_accuracy, is_correct, attempted_at, created_at, updated_at)
	          VALUES (:id, :user_id, :quiz_id, :user_answer, :llm_score, :llm_explanation, :llm_keyword_matches, :llm_completeness, :llm_relevance, :llm_accuracy, :is_correct, :attempted_at, :created_at, :updated_at)`

	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = time.Now()
	}
	attempt.CreatedAt = time.Now()
	attempt.UpdatedAt = time.Now()

	// Handle StringSlice for llm_keyword_matches
	// sqlx handles the Valuer interface of StringSlice automatically.

	_, err := r.db.NamedExecContext(ctx, query, attempt)
	if err != nil {
		return fmt.Errorf("failed to create user quiz attempt: %w", err)
	}
	return nil
}

// buildAttemptsQuery constructs the SELECT query for fetching attempts based on filters and pagination.
// It returns the query string for fetching results, the query string for counting total results, and the arguments map.
func buildAttemptsQuery(baseQueryFields, baseQueryFrom, baseQueryWhere string, userID string, filters dto.AttemptFilters, pagination dto.Pagination, forIncorrectOnly bool) (string, string, map[string]interface{}) {
	args := make(map[string]interface{})
	args["user_id"] = userID

	var whereClauses []string
	if baseQueryWhere != "" {
		whereClauses = append(whereClauses, baseQueryWhere)
	}
	whereClauses = append(whereClauses, "uqa.user_id = :user_id")
	whereClauses = append(whereClauses, "uqa.deleted_at IS NULL")


	if filters.CategoryID != "" {
		// This requires joining with quizzes and sub_categories table
		// Assuming sub_category_id is directly on quizzes table.
		// If filters.CategoryID is for main category, a further join is needed.
		// For now, let's assume filters.CategoryID is actually filters.SubCategoryID for simplicity or that quizzes has category_id
		// This part might need adjustment based on exact DTO and schema relation.
		// Let's assume for now 'q.sub_category_id' can be used and filters.CategoryID is actually sub_category_id
		// Or, if it's a main category ID, the join would be:
		// JOIN quizzes q ON uqa.quiz_id = q.id JOIN sub_categories sc ON q.sub_category_id = sc.id
		// For now, we'll filter by quiz_id if SubCategoryID is provided, assuming a more direct link or pre-filtering.
		// This is a simplification and might need refinement.
		// A better approach would be to have quiz_id in filters or ensure CategoryID allows for proper join.
		// For this example, if CategoryID is present, we assume it means a sub_category_id on the quiz.
		// whereClauses = append(whereClauses, "q.sub_category_id = :category_id") // Requires JOIN with quizzes q
		// args["category_id"] = filters.CategoryID

		// Based on the GetAttemptsByUserID and GetIncorrectAttemptsByUserID, if CategoryID is present,
		// baseQueryFrom is already updated to include JOINs up to sub_categories (sc).
		// So, we can add the filter condition on sc.category_id (if CategoryID is main category)
		// or q.sub_category_id (if CategoryID is sub-category).
		// Let's assume filters.CategoryID is the ID of the *main* category.
		// The JOIN in the calling functions is: user_quiz_attempts uqa JOIN quizzes q ON uqa.quiz_id = q.id JOIN sub_categories sc ON q.sub_category_id = sc.id
		whereClauses = append(whereClauses, "sc.category_id = :category_id")
		args["category_id"] = filters.CategoryID

	}

	if !filters.StartDate.IsZero() {
		whereClauses = append(whereClauses, "uqa.attempted_at >= :start_date")
		args["start_date"] = filters.StartDate
	}
	if !filters.EndDate.IsZero() {
		whereClauses = append(whereClauses, "uqa.attempted_at <= :end_date")
		args["end_date"] = filters.EndDate.Add(24*time.Hour - 1*time.Nanosecond) // Include the whole end day
	}

	if filters.IsCorrect != nil { // Tri-state bool: true, false, or nil (don't filter)
		whereClauses = append(whereClauses, "uqa.is_correct = :is_correct")
		args["is_correct"] = *filters.IsCorrect
	} else if forIncorrectOnly {
		// if baseQueryWhere already sets is_correct = 0, this is redundant but harmless
		// ensure it's not already in baseQueryWhere to avoid "is_correct=0 AND is_correct=0"
		if !strings.Contains(baseQueryWhere, "is_correct = 0") {
			whereClauses = append(whereClauses, "uqa.is_correct = 0") // 0 for false
		}
	}

	queryWhere := ""
	if len(whereClauses) > 0 {
		queryWhere = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	orderBy := "uqa.attempted_at DESC"
	if filters.SortBy != "" {
		allowedSortFields := map[string]string{"attempted_at": "uqa.attempted_at", "score": "uqa.llm_score"}
		dbSortField, ok := allowedSortFields[filters.SortBy]
		if ok {
			orderBy = dbSortField
			if filters.SortOrder != "" && (strings.ToUpper(filters.SortOrder) == "ASC" || strings.ToUpper(filters.SortOrder) == "DESC") {
				orderBy += " " + strings.ToUpper(filters.SortOrder)
			} else {
				orderBy += " DESC"
			}
		}
	}

	limit := pagination.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := pagination.Offset
	if offset < 0 {
		offset = 0
	}

    args["limit"] = limit
    args["offset"] = offset

	resultsQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY %s OFFSET :offset ROWS FETCH NEXT :limit ROWS ONLY", baseQueryFields, baseQueryFrom, queryWhere, orderBy)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", baseQueryFrom, queryWhere)

	return resultsQuery, countQuery, args
}


// GetAttemptsByUserID retrieves a paginated list of quiz attempts for a user, with filters.
func (r *sqlxUserQuizAttemptRepository) GetAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]models.UserQuizAttempt, int, error) {
	baseQueryFields := "uqa.*"
	baseQueryFrom := "user_quiz_attempts uqa"
	baseQueryWhere := ""

	if filters.CategoryID != "" {
		// Assuming filters.CategoryID refers to the main category ID (categories.id)
		// And sub_categories has a category_id FK to categories.id
		// And quizzes has a sub_category_id FK to sub_categories.id
		baseQueryFrom = "user_quiz_attempts uqa JOIN quizzes q ON uqa.quiz_id = q.id JOIN sub_categories sc ON q.sub_category_id = sc.id"
	}

	resultsQuery, countQuery, args := buildAttemptsQuery(baseQueryFields, baseQueryFrom, baseQueryWhere, userID, filters, pagination, false)

	var attempts []models.UserQuizAttempt
	// For SelectContext, if using structs in `args` for nested properties (like `pagination.Limit`), it won't work directly.
	// Ensure `args` is a flat map[string]interface{}. buildAttemptsQuery already does this.
	nstmt, err := r.db.PrepareNamedContext(ctx, resultsQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetAttemptsByUserID results: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}
	defer nstmt.Close()
	err = nstmt.SelectContext(ctx, &attempts, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []models.UserQuizAttempt{}, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to get user quiz attempts: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}

	var total int
	// For GetContext with count query
	nstmtCount, err := r.db.PrepareNamedContext(ctx, countQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetAttemptsByUserID count: %w. Query: %s, Args: %+v", err, countQuery, args)
	}
	defer nstmtCount.Close()
	err = nstmtCount.GetContext(ctx, &total, args)

	if err != nil {
		// sql.ErrNoRows is not expected for COUNT(*), but handle defensively
		return nil, 0, fmt.Errorf("failed to count user quiz attempts: %w. Query: %s, Args: %+v", err, countQuery, args)
	}

	return attempts, total, nil
}

// GetIncorrectAttemptsByUserID retrieves a paginated list of incorrect quiz attempts for a user.
func (r *sqlxUserQuizAttemptRepository) GetIncorrectAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]models.UserQuizAttempt, int, error) {
	baseQueryFields := "uqa.*"
	baseQueryFrom := "user_quiz_attempts uqa"
	baseQueryWhere := "uqa.is_correct = 0"

    if filters.CategoryID != "" {
		baseQueryFrom = "user_quiz_attempts uqa JOIN quizzes q ON uqa.quiz_id = q.id JOIN sub_categories sc ON q.sub_category_id = sc.id"
    }

	resultsQuery, countQuery, args := buildAttemptsQuery(baseQueryFields, baseQueryFrom, baseQueryWhere, userID, filters, pagination, true)

	var attempts []models.UserQuizAttempt
	nstmt, err := r.db.PrepareNamedContext(ctx, resultsQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetIncorrectAttemptsByUserID results: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}
	defer nstmt.Close()
	err = nstmt.SelectContext(ctx, &attempts, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []models.UserQuizAttempt{}, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to get incorrect user quiz attempts: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}

	var total int
	nstmtCount, err := r.db.PrepareNamedContext(ctx, countQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetIncorrectAttemptsByUserID count: %w. Query: %s, Args: %+v", err, countQuery, args)
	}
	defer nstmtCount.Close()
	err = nstmtCount.GetContext(ctx, &total, args)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count incorrect user quiz attempts: %w. Query: %s, Args: %+v", err, countQuery, args)
	}

	return attempts, total, nil
}
