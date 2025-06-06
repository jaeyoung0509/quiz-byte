package repository

import (
	"context"
	"database/sql"
	"fmt"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto" // For DTOs like AttemptFilters, Pagination
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util" // For sql_helpers if needed, though direct sql.Null types are used here
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// sqlxUserQuizAttemptRepository implements domain.UserQuizAttemptRepository using sqlx.
type sqlxUserQuizAttemptRepository struct {
	db *sqlx.DB
}

// NewSQLXUserQuizAttemptRepository creates a new instance of sqlxUserQuizAttemptRepository.
func NewSQLXUserQuizAttemptRepository(db *sqlx.DB) domain.UserQuizAttemptRepository {
	return &sqlxUserQuizAttemptRepository{db: db}
}

func toDomainUserQuizAttempt(modelAttempt *models.UserQuizAttempt) *domain.UserQuizAttempt {
	if modelAttempt == nil {
		return nil
	}
	var deletedAt *time.Time
	if modelAttempt.DeletedAt.Valid {
		deletedAt = &modelAttempt.DeletedAt.Time
	}

	// LLMKeywordMatches is models.StringSlice, needs conversion to []string
	// The StringSlice type itself is []string, so direct assignment is fine.
	var llmKeywordMatches []string
	if modelAttempt.LlmKeywordMatches != nil {
		llmKeywordMatches = modelAttempt.LlmKeywordMatches
	} else {
		llmKeywordMatches = []string{} // Ensure it's not nil for domain model if that's a requirement
	}

	return &domain.UserQuizAttempt{
		ID:                modelAttempt.ID,
		UserID:            modelAttempt.UserID,
		QuizID:            modelAttempt.QuizID,
		UserAnswer:        modelAttempt.UserAnswer.String, // Handle NullString
		LLMScore:          modelAttempt.LlmScore.Float64,  // Handle NullFloat64
		LLMExplanation:    modelAttempt.LlmExplanation.String,
		LLMKeywordMatches: llmKeywordMatches,
		LLMCompleteness:   modelAttempt.LlmCompleteness.Float64,
		LLMRelevance:      modelAttempt.LlmRelevance.Float64,
		LLMAccuracy:       modelAttempt.LlmAccuracy.Float64,
		IsCorrect:         modelAttempt.IsCorrect,
		AttemptedAt:       modelAttempt.AttemptedAt,
		CreatedAt:         modelAttempt.CreatedAt,
		UpdatedAt:         modelAttempt.UpdatedAt,
		DeletedAt:         deletedAt,
	}
}

func fromDomainUserQuizAttempt(domainAttempt *domain.UserQuizAttempt) *models.UserQuizAttempt {
	if domainAttempt == nil {
		return nil
	}
	var deletedAt sql.NullTime
	if domainAttempt.DeletedAt != nil {
		deletedAt = util.TimeToNullTime(*domainAttempt.DeletedAt)
	}

	// LLMKeywordMatches is []string, needs conversion to models.StringSlice
	// models.StringSlice is type []string, so direct assignment.
	var llmKeywordMatches models.StringSlice
	if domainAttempt.LLMKeywordMatches != nil {
		llmKeywordMatches = domainAttempt.LLMKeywordMatches
	} else {
		llmKeywordMatches = models.StringSlice{}
	}

	return &models.UserQuizAttempt{
		ID:                domainAttempt.ID,
		UserID:            domainAttempt.UserID,
		QuizID:            domainAttempt.QuizID,
		UserAnswer:        util.StringToNullString(domainAttempt.UserAnswer),
		LlmScore:          sql.NullFloat64{Float64: domainAttempt.LLMScore, Valid: true}, // Consider if 0 is a valid score or means null
		LlmExplanation:    util.StringToNullString(domainAttempt.LLMExplanation),
		LlmKeywordMatches: llmKeywordMatches,
		LlmCompleteness:   sql.NullFloat64{Float64: domainAttempt.LLMCompleteness, Valid: true},
		LlmRelevance:      sql.NullFloat64{Float64: domainAttempt.LLMRelevance, Valid: true},
		LlmAccuracy:       sql.NullFloat64{Float64: domainAttempt.LLMAccuracy, Valid: true},
		IsCorrect:         domainAttempt.IsCorrect,
		AttemptedAt:       domainAttempt.AttemptedAt,
		CreatedAt:         domainAttempt.CreatedAt,
		UpdatedAt:         domainAttempt.UpdatedAt,
		DeletedAt:         deletedAt,
	}
}

// CreateAttempt inserts a new quiz attempt into the database.
func (r *sqlxUserQuizAttemptRepository) CreateAttempt(ctx context.Context, domainAttempt *domain.UserQuizAttempt) error {
	modelAttempt := fromDomainUserQuizAttempt(domainAttempt)

	if modelAttempt.AttemptedAt.IsZero() {
		modelAttempt.AttemptedAt = time.Now()
	}
	if modelAttempt.CreatedAt.IsZero() {
		modelAttempt.CreatedAt = time.Now()
	}
	modelAttempt.UpdatedAt = time.Now()

	query := `INSERT INTO user_quiz_attempts (ID, USER_ID, QUIZ_ID, USER_ANSWER, LLM_SCORE, LLM_EXPLANATION, LLM_KEYWORD_MATCHES, LLM_COMPLETENESS, LLM_RELEVANCE, LLM_ACCURACY, IS_CORRECT, ATTEMPTED_AT, CREATED_AT, UPDATED_AT, DELETED_AT)
	          VALUES (:1, :2, :3, :4, :5, :6, :7, :8, :9, :10, :11, :12, :13, :14, :15)`

	// Convert StringSlice to string manually for Oracle compatibility
	var keywordMatchesStr string
	if modelAttempt.LlmKeywordMatches != nil {
		keywordMatchesVal, err := modelAttempt.LlmKeywordMatches.Value()
		if err != nil {
			return fmt.Errorf("failed to convert keyword matches to string: %w", err)
		}
		if val, ok := keywordMatchesVal.(string); ok {
			keywordMatchesStr = val
		}
	}

	_, err := r.db.ExecContext(ctx, query,
		modelAttempt.ID,
		modelAttempt.UserID,
		modelAttempt.QuizID,
		modelAttempt.UserAnswer,
		modelAttempt.LlmScore,
		modelAttempt.LlmExplanation,
		keywordMatchesStr, // Use converted string instead of StringSlice
		modelAttempt.LlmCompleteness,
		modelAttempt.LlmRelevance,
		modelAttempt.LlmAccuracy,
		modelAttempt.IsCorrect,
		modelAttempt.AttemptedAt,
		modelAttempt.CreatedAt,
		modelAttempt.UpdatedAt,
		modelAttempt.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user quiz attempt: %w", err)
	}
	return nil
}

// buildAttemptsQuery constructs the SELECT query for fetching attempts based on filters and pagination.
// It returns the query string for fetching results, the query string for counting total results, and the ordered arguments slice.
// Updated for Oracle compatibility using positional parameters.
func buildAttemptsQuery(baseQueryFields, baseQueryFrom, baseQueryWhere string, userID string, filters dto.AttemptFilters, pagination dto.Pagination, forIncorrectOnly bool) (string, string, []interface{}) {
	var args []interface{}
	var whereClauses []string
	argIndex := 1

	if baseQueryWhere != "" {
		whereClauses = append(whereClauses, baseQueryWhere)
	}

	whereClauses = append(whereClauses, fmt.Sprintf("uqa.user_id = :%d", argIndex))
	args = append(args, userID)
	argIndex++

	whereClauses = append(whereClauses, "uqa.deleted_at IS NULL")

	if filters.CategoryID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("sc.category_id = :%d", argIndex))
		args = append(args, filters.CategoryID)
		argIndex++
	}

	if filters.StartDate != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("uqa.attempted_at >= :%d", argIndex))
		args = append(args, filters.StartDate)
		argIndex++
	}
	if filters.EndDate != "" {
		parsedEndDate, err := time.Parse("2006-01-02", filters.EndDate)
		if err == nil {
			whereClauses = append(whereClauses, fmt.Sprintf("uqa.attempted_at <= :%d", argIndex))
			args = append(args, parsedEndDate.Add(24*time.Hour-1*time.Nanosecond))
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("uqa.attempted_at <= :%d", argIndex))
			args = append(args, filters.EndDate)
		}
		argIndex++
	}

	if filters.IsCorrect != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("uqa.is_correct = :%d", argIndex))
		args = append(args, *filters.IsCorrect)
		argIndex++
	} else if forIncorrectOnly {
		if !strings.Contains(baseQueryWhere, "is_correct = 0") && !strings.Contains(baseQueryWhere, "is_correct") {
			whereClauses = append(whereClauses, "uqa.is_correct = 0")
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

	// Oracle compatibility: Use ROW_NUMBER() approach with positional parameters only
	innerQuery := fmt.Sprintf("SELECT %s, ROW_NUMBER() OVER (ORDER BY %s) as rn FROM %s %s", baseQueryFields, orderBy, baseQueryFrom, queryWhere)
	resultsQuery := fmt.Sprintf("SELECT * FROM (%s) WHERE rn > %d AND rn <= %d", innerQuery, offset, offset+limit)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", baseQueryFrom, queryWhere)

	return resultsQuery, countQuery, args
}

// GetAttemptsByUserID retrieves a paginated list of quiz attempts for a user, with filters.
func (r *sqlxUserQuizAttemptRepository) GetAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]domain.UserQuizAttempt, int, error) {
	baseQueryFields := "uqa.*"
	baseQueryFrom := "user_quiz_attempts uqa"
	baseQueryWhere := ""

	if filters.CategoryID != "" {
		baseQueryFrom = "user_quiz_attempts uqa JOIN quizzes q ON uqa.quiz_id = q.id JOIN sub_categories sc ON q.sub_category_id = sc.id"
	}

	resultsQuery, countQuery, args := buildAttemptsQuery(baseQueryFields, baseQueryFrom, baseQueryWhere, userID, filters, pagination, false)

	var modelAttempts []models.UserQuizAttempt
	rows, err := r.db.QueryContext(ctx, resultsQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute query for GetAttemptsByUserID results: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}
	defer rows.Close()

	for rows.Next() {
		var ma models.UserQuizAttempt
		var rn int // Row number column from ROW_NUMBER()
		if err := rows.Scan(
			&ma.ID,
			&ma.UserID,
			&ma.QuizID,
			&ma.UserAnswer,
			&ma.LlmScore,
			&ma.LlmExplanation,
			&ma.LlmKeywordMatches,
			&ma.LlmCompleteness,
			&ma.LlmRelevance,
			&ma.LlmAccuracy,
			&ma.IsCorrect,
			&ma.AttemptedAt,
			&ma.CreatedAt,
			&ma.UpdatedAt,
			&ma.DeletedAt,
			&rn, // Row number column
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user quiz attempt: %w", err)
		}
		modelAttempts = append(modelAttempts, ma)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate user quiz attempts: %w", err)
	}

	domainAttempts := make([]domain.UserQuizAttempt, len(modelAttempts))
	for i, ma := range modelAttempts {
		da := toDomainUserQuizAttempt(&ma) // Pass pointer to ma
		if da != nil {
			domainAttempts[i] = *da
		}
		// Handle case where da is nil if necessary, though toDomainUserQuizAttempt shouldn't return nil for non-nil input
	}

	var total int
	countRows, err := r.db.QueryContext(ctx, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute count query for GetAttemptsByUserID: %w. Query: %s, Args: %+v", err, countQuery, args)
	}
	defer countRows.Close()

	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("failed to scan count result: %w", err)
		}
	}

	if err := countRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to get count: %w", err)
	}

	return domainAttempts, total, nil
}

// GetIncorrectAttemptsByUserID retrieves a paginated list of incorrect quiz attempts for a user.
func (r *sqlxUserQuizAttemptRepository) GetIncorrectAttemptsByUserID(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) ([]domain.UserQuizAttempt, int, error) {
	baseQueryFields := "uqa.*"
	baseQueryFrom := "user_quiz_attempts uqa"
	// Ensure the base query correctly filters for incorrect attempts.
	// The `forIncorrectOnly` flag in `buildAttemptsQuery` will also add `uqa.is_correct = 0`
	// if not already present in `baseQueryWhere` and `filters.IsCorrect` is nil.
	baseQueryWhere := "uqa.is_correct = 0"

	if filters.CategoryID != "" {
		baseQueryFrom = "user_quiz_attempts uqa JOIN quizzes q ON uqa.quiz_id = q.id JOIN sub_categories sc ON q.sub_category_id = sc.id"
	}

	resultsQuery, countQuery, args := buildAttemptsQuery(baseQueryFields, baseQueryFrom, baseQueryWhere, userID, filters, pagination, true)

	var modelAttempts []models.UserQuizAttempt
	rows, err := r.db.QueryContext(ctx, resultsQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute query for GetIncorrectAttemptsByUserID results: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}
	defer rows.Close()

	for rows.Next() {
		var ma models.UserQuizAttempt
		var rn int // Row number column from ROW_NUMBER()
		if err := rows.Scan(
			&ma.ID,
			&ma.UserID,
			&ma.QuizID,
			&ma.UserAnswer,
			&ma.LlmScore,
			&ma.LlmExplanation,
			&ma.LlmKeywordMatches,
			&ma.LlmCompleteness,
			&ma.LlmRelevance,
			&ma.LlmAccuracy,
			&ma.IsCorrect,
			&ma.AttemptedAt,
			&ma.CreatedAt,
			&ma.UpdatedAt,
			&ma.DeletedAt,
			&rn, // Row number column
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user quiz attempt: %w", err)
		}
		modelAttempts = append(modelAttempts, ma)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate user quiz attempts: %w", err)
	}

	domainAttempts := make([]domain.UserQuizAttempt, len(modelAttempts))
	for i, ma := range modelAttempts {
		da := toDomainUserQuizAttempt(&ma) // Pass pointer to ma
		if da != nil {
			domainAttempts[i] = *da
		}
	}

	var total int
	countRows, err := r.db.QueryContext(ctx, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute count query for GetIncorrectAttemptsByUserID: %w. Query: %s, Args: %+v", err, countQuery, args)
	}
	defer countRows.Close()

	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("failed to scan count result: %w", err)
		}
	}

	if err := countRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to get count: %w", err)
	}

	return domainAttempts, total, nil
}
