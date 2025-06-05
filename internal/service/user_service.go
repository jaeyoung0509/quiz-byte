package service

import (
	"context"
	"database/sql" // For sql.NullString checks if needed, though models handle it
	"errors"
	"fmt"
	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/repository"
	"quiz-byte/internal/repository/models"
	"quiz-byte/internal/util" // For ULID or other utils if needed
	"time"
)

var (
	ErrUserProfileNotFound = errors.New("user profile not found")
	ErrQuizDetailNotFound  = errors.New("quiz detail not found for an attempt")
)

const DefaultCorrectnessThreshold = 0.7 // Example threshold

// UserService defines the interface for user-related operations.
type UserService interface {
	GetUserProfile(ctx context.Context, userID string) (*dto.UserProfileResponse, error)
	RecordQuizAttempt(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error
	GetUserQuizAttempts(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserQuizAttemptsResponse, error)
	GetUserIncorrectAnswers(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserIncorrectAnswersResponse, error)
	GetUserRecommendations(ctx context.Context, userID string, limit int, optionalSubCategoryID string) (*dto.QuizRecommendationsResponse, error)
}

type userServiceImpl struct {
	userRepo    repository.UserRepository
	attemptRepo repository.UserQuizAttemptRepository
	quizRepo    domain.QuizRepository // Changed from repository.QuizRepository
	appConfig   *config.Config
}

// NewUserService creates a new instance of UserService.
func NewUserService(
	userRepo repository.UserRepository,
	attemptRepo repository.UserQuizAttemptRepository,
	quizRepo domain.QuizRepository, // Changed from repository.QuizRepository
	appConfig *config.Config,
) UserService {
	return &userServiceImpl{
		userRepo:    userRepo,
		attemptRepo: attemptRepo,
		quizRepo:    quizRepo, // Changed
		appConfig:   appConfig,
	}
}

// GetUserProfile retrieves a user's profile information.
func (s *userServiceImpl) GetUserProfile(ctx context.Context, userID string) (*dto.UserProfileResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		// Check if the error is because the user was not found.
		// The repository's GetUserByID might return (nil, nil) for not found,
		// or a specific error type that can be checked with errors.Is().
		// Assuming the repository returns (nil, nil) for not found as per common practice in this project.
		if user == nil && err == nil { // This condition implies not found from repo's (nil,nil)
			return nil, ErrUserProfileNotFound
		}
		// If err is not nil, it's some other repository error.
		return nil, fmt.Errorf("failed to get user by id from repository: %w", err)
	}

	return &dto.UserProfileResponse{
		ID:                user.ID,
		Email:             user.Email,
		Name:              user.Name.String,              // Assuming Name is sql.NullString
		ProfilePictureURL: user.ProfilePictureURL.String, // Assuming ProfilePictureURL is sql.NullString
	}, nil
}

// RecordQuizAttempt records a user's quiz attempt.
func (s *userServiceImpl) RecordQuizAttempt(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error {
	if evalResult == nil {
		return errors.New("evaluation result cannot be nil")
	}

	isCorrect := evalResult.Score >= DefaultCorrectnessThreshold // Determine correctness

	// Assuming evalResult.KeywordMatches is []string, needs conversion to models.StringSlice
	var keywordMatchesSlice models.StringSlice
	if evalResult.KeywordMatches != nil {
		keywordMatchesSlice = models.StringSlice(evalResult.KeywordMatches)
	}

	attempt := &models.UserQuizAttempt{
		ID:                util.NewULID(),
		UserID:            userID,
		QuizID:            quizID,
		UserAnswer:        util.StringToNullString(userAnswer),
		LlmScore:          sql.NullFloat64{Float64: evalResult.Score, Valid: true},
		LlmExplanation:    util.StringToNullString(evalResult.Explanation),
		LlmKeywordMatches: keywordMatchesSlice, // Converted from []string
		LlmCompleteness:   sql.NullFloat64{Float64: evalResult.Completeness, Valid: true},
		LlmRelevance:      sql.NullFloat64{Float64: evalResult.Relevance, Valid: true},
		LlmAccuracy:       sql.NullFloat64{Float64: evalResult.Accuracy, Valid: true},
		IsCorrect:         isCorrect,
		AttemptedAt:       evalResult.AnsweredAt,
	}
	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = time.Now()
	}

	if err := s.attemptRepo.CreateAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("failed to create user quiz attempt in repository: %w", err)
	}
	return nil
}

// GetUserQuizAttempts retrieves a user's quiz attempt history.
func (s *userServiceImpl) GetUserQuizAttempts(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserQuizAttemptsResponse, error) {
	attempts, total, err := s.attemptRepo.GetAttemptsByUserID(ctx, userID, filters, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quiz attempts from repository: %w", err)
	}

	attemptItems := make([]dto.UserQuizAttemptItem, len(attempts))
	for i, attempt := range attempts {
		quiz, errQuiz := s.quizRepo.GetQuizByID(attempt.QuizID)
		if errQuiz != nil || quiz == nil {
			if quiz == nil && errQuiz == nil { // Repository returned (nil,nil) for not found
				return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s (quiz not found)", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID)
			}
			return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s, repo error: %v", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID, errQuiz)
		}

		attemptItems[i] = dto.UserQuizAttemptItem{
			AttemptID:      attempt.ID,
			QuizID:         attempt.QuizID,
			QuizQuestion:   quiz.Question,
			UserAnswer:     attempt.UserAnswer.String,
			LlmScore:       attempt.LlmScore.Float64,
			LlmExplanation: attempt.LlmExplanation.String,
			IsCorrect:      attempt.IsCorrect,
			AttemptedAt:    attempt.AttemptedAt,
		}
	}

	currentPage := 0
	totalPages := 0
	if pagination.Limit > 0 {
		currentPage = pagination.Offset/pagination.Limit + 1
		totalPages = (total + pagination.Limit - 1) / pagination.Limit
	}

	return &dto.UserQuizAttemptsResponse{
		Attempts: attemptItems,
		PaginationInfo: dto.PaginationInfo{
			TotalItems:  total,
			Limit:       pagination.Limit,
			Offset:      pagination.Offset,
			CurrentPage: currentPage,
			TotalPages:  totalPages,
		},
	}, nil
}

// GetUserRecommendations retrieves a list of recommended quizzes for the user.
// Currently, it recommends unattempted quizzes.
func (s *userServiceImpl) GetUserRecommendations(ctx context.Context, userID string, limit int, optionalSubCategoryID string) (*dto.QuizRecommendationsResponse, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	recommendationItems, err := s.quizRepo.GetUnattemptedQuizzesWithDetails(ctx, userID, limit, optionalSubCategoryID)
	if err != nil {
		// logger.Get().Error("Failed to get unattempted quizzes for recommendations", zap.String("userID", userID), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve recommendations: %w", err)
	}

	return &dto.QuizRecommendationsResponse{
		Recommendations: recommendationItems,
	}, nil
}

// GetUserIncorrectAnswers retrieves a user's incorrect answers.
func (s *userServiceImpl) GetUserIncorrectAnswers(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserIncorrectAnswersResponse, error) {
	isCorrectFilter := false
	filters.IsCorrect = &isCorrectFilter

	attempts, total, err := s.attemptRepo.GetIncorrectAttemptsByUserID(ctx, userID, filters, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to get user incorrect answers from repository: %w", err)
	}

	incorrectAnswerItems := make([]dto.UserIncorrectAnswerItem, len(attempts))
	for i, attempt := range attempts {
		quiz, errQuiz := s.quizRepo.GetQuizByID(attempt.QuizID)
		if errQuiz != nil || quiz == nil {
			if quiz == nil && errQuiz == nil {
				return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s (quiz not found)", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID)
			}
			return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s, repo error: %v", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID, errQuiz)
		}

		incorrectAnswerItems[i] = dto.UserIncorrectAnswerItem{
			AttemptID:      attempt.ID,
			QuizID:         attempt.QuizID,
			QuizQuestion:   quiz.Question,
			UserAnswer:     attempt.UserAnswer.String,
			CorrectAnswer:  quiz.ModelAnswers[0], // Assuming ModelAnswers is a slice and we take the first one as the correct answer  TODO: fix it
			LlmScore:       attempt.LlmScore.Float64,
			LlmExplanation: attempt.LlmExplanation.String,
			AttemptedAt:    attempt.AttemptedAt,
			// QuizExplanation: quiz.Explanation,
		}
	}

	currentPage := 0
	totalPages := 0
	if pagination.Limit > 0 {
		currentPage = pagination.Offset/pagination.Limit + 1
		totalPages = (total + pagination.Limit - 1) / pagination.Limit
	}

	return &dto.UserIncorrectAnswersResponse{
		IncorrectAnswers: incorrectAnswerItems,
		PaginationInfo: dto.PaginationInfo{
			TotalItems:  total,
			Limit:       pagination.Limit,
			Offset:      pagination.Offset,
			CurrentPage: currentPage,
			TotalPages:  totalPages,
		},
	}, nil
}
