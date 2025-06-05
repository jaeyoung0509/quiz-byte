package service

import (
	"context"
	"errors"
	"fmt"
	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/dto"
	"quiz-byte/internal/util" // For ULID or other utils if needed
	"time"
	// No longer directly using repository or models, only domain interfaces
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
	userRepo    domain.UserRepository           // Changed
	attemptRepo domain.UserQuizAttemptRepository // Changed
	quizRepo    domain.QuizRepository
	// appConfig   *config.Config // Removed
}

// NewUserService creates a new instance of UserService.
func NewUserService(
	userRepo domain.UserRepository,           // Changed
	attemptRepo domain.UserQuizAttemptRepository, // Changed
	quizRepo domain.QuizRepository,
	// appConfig *config.Config, // Removed
) UserService {
	return &userServiceImpl{
		userRepo:    userRepo,
		attemptRepo: attemptRepo,
		quizRepo:    quizRepo,
		// appConfig:   appConfig, // Removed
	}
}

// GetUserProfile retrieves a user's profile information.
func (s *userServiceImpl) GetUserProfile(ctx context.Context, userID string) (*dto.UserProfileResponse, error) {
	domainUser, err := s.userRepo.GetUserByID(ctx, userID) // Returns domain.User
	if err != nil {
		// Check if the error is because the user was not found.
		// The domain repository's GetUserByID should ideally return a specific domain.NotFoundError
		if errors.Is(err, &domain.NotFoundError{}) || domainUser == nil && err == nil { // Adapt to how not found is signaled
			return nil, ErrUserProfileNotFound
		}
		return nil, fmt.Errorf("failed to get user by id from repository: %w", err)
	}
	if domainUser == nil { // Explicitly handle case where err is nil but user is nil (should be covered by NotFoundError)
		return nil, ErrUserProfileNotFound
	}

	return &dto.UserProfileResponse{
		ID:                domainUser.ID,
		Email:             domainUser.Email,
		Name:              domainUser.Name, // domain.User.Name is string
		ProfilePictureURL: domainUser.ProfilePictureURL, // domain.User.ProfilePictureURL is string
	}, nil
}

// RecordQuizAttempt records a user's quiz attempt.
func (s *userServiceImpl) RecordQuizAttempt(ctx context.Context, userID string, quizID string, userAnswer string, evalResult *domain.Answer) error {
	if evalResult == nil {
		return errors.New("evaluation result cannot be nil")
	}

	isCorrect := evalResult.Score >= DefaultCorrectnessThreshold

	// domain.UserQuizAttempt uses []string for LLMKeywordMatches
	// evalResult.KeywordMatches is already []string from domain.Answer
	var llmKeywordMatches []string
	if evalResult.KeywordMatches != nil {
		llmKeywordMatches = evalResult.KeywordMatches
	} else {
		llmKeywordMatches = []string{}
	}


	domainAttempt := &domain.UserQuizAttempt{ // Changed to domain.UserQuizAttempt
		ID:                util.NewULID(),
		UserID:            userID,
		QuizID:            quizID,
		UserAnswer:        userAnswer, // domain.UserQuizAttempt.UserAnswer is string
		LLMScore:          evalResult.Score,
		LLMExplanation:    evalResult.Explanation,
		LLMKeywordMatches: llmKeywordMatches,
		LLMCompleteness:   evalResult.Completeness,
		LLMRelevance:      evalResult.Relevance,
		LLMAccuracy:       evalResult.Accuracy,
		IsCorrect:         isCorrect,
		AttemptedAt:       evalResult.AnsweredAt,
		// CreatedAt and UpdatedAt will be set by repository or domain constructor if applicable
	}
	if domainAttempt.AttemptedAt.IsZero() {
		domainAttempt.AttemptedAt = time.Now()
	}

	if err := s.attemptRepo.CreateAttempt(ctx, domainAttempt); err != nil { // Call with domain.UserQuizAttempt
		return fmt.Errorf("failed to create user quiz attempt in repository: %w", err)
	}
	return nil
}

// GetUserQuizAttempts retrieves a user's quiz attempt history.
func (s *userServiceImpl) GetUserQuizAttempts(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserQuizAttemptsResponse, error) {
	domainAttempts, total, err := s.attemptRepo.GetAttemptsByUserID(ctx, userID, filters, pagination) // Returns []domain.UserQuizAttempt
	if err != nil {
		return nil, fmt.Errorf("failed to get user quiz attempts from repository: %w", err)
	}

	attemptItems := make([]dto.UserQuizAttemptItem, len(domainAttempts))
	for i, attempt := range domainAttempts { // attempt is domain.UserQuizAttempt
		quiz, errQuiz := s.quizRepo.GetQuizByID(attempt.QuizID)
		if errQuiz != nil || quiz == nil {
			if quiz == nil && errQuiz == nil {
				return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s (quiz not found)", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID)
			}
			return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s, repo error: %v", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID, errQuiz)
		}

		attemptItems[i] = dto.UserQuizAttemptItem{
			AttemptID:      attempt.ID,
			QuizID:         attempt.QuizID,
			QuizQuestion:   quiz.Question,
			UserAnswer:     attempt.UserAnswer, // Direct from domain.UserQuizAttempt
			LlmScore:       attempt.LLMScore,
			LlmExplanation: attempt.LLMExplanation,
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
		return nil, fmt.Errorf("failed to retrieve recommendations: %w", err)
	}

	return &dto.QuizRecommendationsResponse{
		Recommendations: recommendationItems,
	}, nil
}

// GetUserIncorrectAnswers retrieves a user's incorrect answers.
func (s *userServiceImpl) GetUserIncorrectAnswers(ctx context.Context, userID string, filters dto.AttemptFilters, pagination dto.Pagination) (*dto.UserIncorrectAnswersResponse, error) {
	isCorrectFilter := false
	filters.IsCorrect = &isCorrectFilter // This DTO field is a *bool

	domainAttempts, total, err := s.attemptRepo.GetIncorrectAttemptsByUserID(ctx, userID, filters, pagination) // Returns []domain.UserQuizAttempt
	if err != nil {
		return nil, fmt.Errorf("failed to get user incorrect answers from repository: %w", err)
	}

	incorrectAnswerItems := make([]dto.UserIncorrectAnswerItem, len(domainAttempts))
	for i, attempt := range domainAttempts { // attempt is domain.UserQuizAttempt
		quiz, errQuiz := s.quizRepo.GetQuizByID(attempt.QuizID)
		if errQuiz != nil || quiz == nil {
			if quiz == nil && errQuiz == nil {
				return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s (quiz not found)", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID)
			}
			return nil, fmt.Errorf("%w: quiz_id %s for attempt_id %s, repo error: %v", ErrQuizDetailNotFound, attempt.QuizID, attempt.ID, errQuiz)
		}

		// Note: domain.Quiz.ModelAnswers is a string, not []string.
		// If it's a JSON array string or delimited, it needs parsing.
		// For simplicity, assuming it's a single answer or the DTO needs to be adapted.
		// The original code had quiz.ModelAnswers[0] - this will break if ModelAnswers is just a string.
		// For now, I'll pass quiz.ModelAnswers directly. This might need further review based on actual data.
		correctAnswer := quiz.ModelAnswers // This was quiz.ModelAnswers[0]
		if len(quiz.ModelAnswers) > 0 && (quiz.ModelAnswers[0] == '[' || quiz.ModelAnswers[0] == '{') {
			// Simple check if it might be JSON array/object, could be more robust.
			// Or if it's a specific delimited string.
			// For this task, we'll assume the DTO might expect a single string or this needs external adjustment.
		}


		incorrectAnswerItems[i] = dto.UserIncorrectAnswerItem{
			AttemptID:      attempt.ID,
			QuizID:         attempt.QuizID,
			QuizQuestion:   quiz.Question,
			UserAnswer:     attempt.UserAnswer, // Direct from domain.UserQuizAttempt
			CorrectAnswer:  correctAnswer,      // Changed from attempt.CorrectAnswer
			LlmScore:       attempt.LLMScore,
			LlmExplanation: attempt.LLMExplanation,
			AttemptedAt:    attempt.AttemptedAt,
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
