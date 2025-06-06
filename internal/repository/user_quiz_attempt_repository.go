package repository

import (
	"context"
	"database/sql"
	"errors"
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

	query := `INSERT INTO user_quiz_attempts (id, user_id, quiz_id, user_answer, llm_score, llm_explanation, llm_keyword_matches, llm_completeness, llm_relevance, llm_accuracy, is_correct, attempted_at, created_at, updated_at, deleted_at)
	          VALUES (:id, :user_id, :quiz_id, :user_answer, :llm_score, :llm_explanation, :llm_keyword_matches, :llm_completeness, :llm_relevance, :llm_accuracy, :is_correct, :attempted_at, :created_at, :updated_at, :deleted_at)`

	_, err := r.db.NamedExecContext(ctx, query, modelAttempt)
	if err != nil {
		return fmt.Errorf("failed to create user quiz attempt: %w", err)
	}
	return nil
}

// buildAttemptsQuery constructs the SELECT query for fetching attempts based on filters and pagination.
// It returns the query string for fetching results, the query string for counting total results, and the arguments map.
// This function remains unchanged as it operates on model level and DTOs.
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
		whereClauses = append(whereClauses, "sc.category_id = :category_id")
		args["category_id"] = filters.CategoryID
	}

	if filters.StartDate != "" {
		whereClauses = append(whereClauses, "uqa.attempted_at >= :start_date")
		args["start_date"] = filters.StartDate
	}
	if filters.EndDate != "" {
		parsedEndDate, err := time.Parse("2006-01-02", filters.EndDate)
		if err == nil {
			whereClauses = append(whereClauses, "uqa.attempted_at <= :end_date")
			args["end_date"] = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
		} else {
			whereClauses = append(whereClauses, "uqa.attempted_at <= :end_date_str")
			args["end_date_str"] = filters.EndDate
		}
	}

	if filters.IsCorrect != nil {
		whereClauses = append(whereClauses, "uqa.is_correct = :is_correct")
		args["is_correct"] = *filters.IsCorrect
	} else if forIncorrectOnly {
		if !strings.Contains(baseQueryWhere, "is_correct = 0") && !strings.Contains(baseQueryWhere, "is_correct = :is_correct") {
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

	args["limit"] = limit
	args["offset"] = offset

	resultsQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY %s OFFSET :offset ROWS FETCH NEXT :limit ROWS ONLY", baseQueryFields, baseQueryFrom, queryWhere, orderBy)
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
	nstmt, err := r.db.PrepareNamedContext(ctx, resultsQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetAttemptsByUserID results: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}
	defer nstmt.Close()
	err = nstmt.SelectContext(ctx, &modelAttempts, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []domain.UserQuizAttempt{}, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to get user quiz attempts: %w. Query: %s, Args: %+v", err, resultsQuery, args)
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
	nstmtCount, err := r.db.PrepareNamedContext(ctx, countQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetAttemptsByUserID count: %w. Query: %s, Args: %+v", err, countQuery, args)
	}
	defer nstmtCount.Close()
	err = nstmtCount.GetContext(ctx, &total, args)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user quiz attempts: %w. Query: %s, Args: %+v", err, countQuery, args)
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
	nstmt, err := r.db.PrepareNamedContext(ctx, resultsQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to prepare query for GetIncorrectAttemptsByUserID results: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}
	defer nstmt.Close()
	err = nstmt.SelectContext(ctx, &modelAttempts, args)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []domain.UserQuizAttempt{}, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to get incorrect user quiz attempts: %w. Query: %s, Args: %+v", err, resultsQuery, args)
	}

	domainAttempts := make([]domain.UserQuizAttempt, len(modelAttempts))
	for i, ma := range modelAttempts {
		da := toDomainUserQuizAttempt(&ma) // Pass pointer to ma
		if da != nil {
			domainAttempts[i] = *da
		}
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

	return domainAttempts, total, nil
}
