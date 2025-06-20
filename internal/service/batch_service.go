package service

import (
	"context"
	"fmt"  // Keep for basic logging/placeholder, zap will be main
	"time" // For logging timestamps if not implicitly handled by zap

	"quiz-byte/internal/config"
	"quiz-byte/internal/domain"
	"quiz-byte/internal/util" // For CosineSimilarity and NewULID

	"go.uber.org/zap"
)

// batchService implements the domain.BatchService interface.
type batchService struct {
	quizRepo         domain.QuizRepository
	categoryRepo     domain.CategoryRepository
	embeddingService domain.EmbeddingService
	quizGenSvc       domain.QuizGenerationService // Renamed from llmClient
	txManager        domain.TransactionManager
	cfg              *config.Config
	logger           *zap.Logger
}

// NewBatchService creates a new instance of batchService.
func NewBatchService(
	quizRepo domain.QuizRepository,
	categoryRepo domain.CategoryRepository,
	embeddingService domain.EmbeddingService,
	quizGenSvc domain.QuizGenerationService, // Renamed from llmClient
	txManager domain.TransactionManager,
	cfg *config.Config,
	logger *zap.Logger,
) domain.BatchService {
	return &batchService{
		quizRepo:         quizRepo,
		categoryRepo:     categoryRepo,
		embeddingService: embeddingService,
		quizGenSvc:       quizGenSvc, // Renamed from llmClient
		txManager:        txManager,
		cfg:              cfg,
		logger:           logger,
	}
}

// GenerateNewQuizzesAndSave implements the logic to generate and save new quizzes.
func (s *batchService) GenerateNewQuizzesAndSave(ctx context.Context) error {
	s.logger.Info("Starting batch quiz generation process", zap.Time("start_time", time.Now()))

	// 전체 배치 프로세스를 트랜잭션으로 감싸기
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.generateAndSaveQuizzesInTransaction(txCtx)
	})
}

// generateAndSaveQuizzesInTransaction 트랜잭션 내에서 실행되는 실제 배치 로직
func (s *batchService) generateAndSaveQuizzesInTransaction(ctx context.Context) error {

	var existingEmbeddingsCache map[string][]float32

	subCategoryIDs, err := s.quizRepo.GetAllSubCategories(ctx)
	if err != nil {
		s.logger.Error("Failed to fetch subcategory IDs", zap.Error(err))
		return domain.NewInternalError("failed to fetch subcategory IDs", err)
	}

	if len(subCategoryIDs) == 0 {
		s.logger.Info("No subcategories found. Batch process finishing early.")
		return nil
	}

	for _, subCategoryID := range subCategoryIDs {
		existingEmbeddingsCache = make(map[string][]float32) // Reset for each subcategory
		s.logger.Info("Processing subcategory", zap.String("subcategory_id", subCategoryID))

		existingQuizzes, err := s.quizRepo.GetQuizzesBySubCategory(ctx, subCategoryID)
		if err != nil {
			s.logger.Error("Failed to fetch existing quizzes for subcategory",
				zap.String("subcategory_id", subCategoryID),
				zap.Error(err),
			)
			// Continue to the next subcategory, or return error? For now, continue.
			continue
		}
		s.logger.Info("Fetched existing quizzes for subcategory",
			zap.String("subcategory_id", subCategoryID),
			zap.Int("count", len(existingQuizzes)),
		)

		// Build a list of existing question texts (placeholder for now)
		// This list could be passed to the LLM to avoid generating similar questions.
		// var existingQuestionTexts []string
		// for _, q := range existingQuizzes {
		// 	existingQuestionTexts = append(existingQuestionTexts, q.Question)
		// }
		// s.logger.Debug("Existing questions in subcategory", zap.Strings("questions", existingQuestionTexts))

		// TODO: Fetch SubCategoryName based on subCategoryID if needed by LLMClient, or pass ID directly
		// For now, we'll pass subCategoryID as subCategoryName, assuming LLM can work with it or it's just for context.
		subCategoryNameForLLM := subCategoryID // Placeholder, might need actual name

		// Collect existing keywords from the questions in the current subcategory
		var existingKeywordsForLLM []string
		for _, q := range existingQuizzes {
			existingKeywordsForLLM = append(existingKeywordsForLLM, q.Keywords...)
		}
		// Deduplicate keywords if necessary, though for the prompt, some repetition might be okay.

		numQuestionsToGenerate := s.cfg.Batch.NumQuestionsPerSubCategory // Assuming this config exists
		if numQuestionsToGenerate == 0 {
			numQuestionsToGenerate = 2 // Default if not configured
		}

		generatedQuizzesData, err := s.quizGenSvc.GenerateQuizCandidates(ctx, subCategoryNameForLLM, existingKeywordsForLLM, numQuestionsToGenerate)
		if err != nil {
			s.logger.Error("Failed to generate quiz candidates from QuizGenerationService", // Updated log message
				zap.String("subcategory_id", subCategoryID),
				zap.Error(err),
			)
			continue // Skip to the next subcategory
		}

		if len(generatedQuizzesData) == 0 {
			s.logger.Info("LLM returned no quiz data for subcategory", zap.String("subcategory_id", subCategoryID))
			continue
		}

		s.logger.Info("Successfully received quiz candidates from QuizGenerationService", // Updated log message
			zap.String("subcategory_id", subCategoryID),
			zap.Int("num_generated", len(generatedQuizzesData)),
		)

		for _, generatedQuiz := range generatedQuizzesData { // Iterate over pointer to NewQuizData
			if generatedQuiz == nil {
				s.logger.Warn("LLM returned a nil quiz data object, skipping", zap.String("subcategory_id", subCategoryID))
				continue
			}
			s.logger.Info("Processing generated quiz data", zap.String("question", generatedQuiz.Question))

			newQuizEmbedding, err := s.embeddingService.Generate(ctx, generatedQuiz.Question)
			if err != nil {
				s.logger.Error("Failed to generate embedding for new quiz",
					zap.String("question", generatedQuiz.Question), // Corrected generatedQuizData to generatedQuiz
					zap.Error(err),
				)
				continue // Skip this generated quiz
			}

			isUnique := true
			for _, existingQuiz := range existingQuizzes {
				var existingQuizEmbedding []float32
				var errEmbedding error

				if emb, found := existingEmbeddingsCache[existingQuiz.ID]; found {
					existingQuizEmbedding = emb
				} else {
					existingQuizEmbedding, errEmbedding = s.embeddingService.Generate(ctx, existingQuiz.Question)
					if errEmbedding != nil {
						s.logger.Error("Failed to generate embedding for existing quiz",
							zap.String("quiz_id", existingQuiz.ID),
							zap.String("question", existingQuiz.Question),
							zap.Error(errEmbedding),
						)
						// If we can't generate embedding for an existing quiz, we can't compare.
						// Depending on policy, we might skip comparison or assume it's not similar.
						// For now, log and continue (meaning it won't be flagged as non-unique due to this error).
						continue
					}
					existingEmbeddingsCache[existingQuiz.ID] = existingQuizEmbedding
				}

				if len(newQuizEmbedding) == 0 || len(existingQuizEmbedding) == 0 {
					s.logger.Warn("One or both embeddings are empty, skipping similarity check.",
						zap.String("new_quiz_question", generatedQuiz.Question), // Corrected field
						zap.String("existing_quiz_id", existingQuiz.ID),
					)
					continue
				}

				similarity, err := util.CosineSimilarity(newQuizEmbedding, existingQuizEmbedding)
				if err != nil {
					s.logger.Error("Failed to calculate cosine similarity",
						zap.String("new_quiz_question", generatedQuiz.Question), // Corrected field
						zap.String("existing_quiz_id", existingQuiz.ID),
						zap.Error(err),
					)
					// If similarity calculation fails, policy decision:
					// Treat as not similar and proceed, or skip? For now, treat as not similar.
					continue
				}

				s.logger.Debug("Calculated similarity",
					zap.String("new_quiz_question", generatedQuiz.Question), // Corrected field
					zap.String("existing_quiz_id", existingQuiz.ID),
					zap.Float64("similarity", similarity),
					zap.Float64("threshold", s.cfg.Embedding.SimilarityThreshold),
				)

				if similarity >= s.cfg.Embedding.SimilarityThreshold {
					isUnique = false
					s.logger.Info("Generated quiz is too similar to existing quiz",
						zap.String("generated_question", generatedQuiz.Question), // Corrected field
						zap.String("existing_quiz_id", existingQuiz.ID),
						zap.String("existing_quiz_question", existingQuiz.Question),
						zap.Float64("similarity", similarity),
					)
					break // Found a similar existing quiz, no need to check others
				}
			}

			if isUnique {
				newDomainQuiz := domain.Quiz{
					// ID will be set by SaveQuiz or here, let's ensure it's set before save
					ID:            util.NewULID(),
					Question:      generatedQuiz.Question,
					ModelAnswers:  []string{generatedQuiz.ModelAnswer},
					Keywords:      generatedQuiz.Keywords,
					Difficulty:    domain.DifficultyToInt(generatedQuiz.Difficulty),
					SubCategoryID: subCategoryID,
					// CreatedAt and UpdatedAt will be set by the repository
				}

				err = s.quizRepo.SaveQuiz(ctx, &newDomainQuiz) // Pass context
				if err != nil {
					s.logger.Error("Failed to save new unique quiz",
						zap.String("question", newDomainQuiz.Question),
						zap.Error(err),
					)
					// Continue to next generated quiz even if one save fails
				} else {
					s.logger.Info("Successfully saved new unique quiz",
						zap.String("quiz_id", newDomainQuiz.ID),
						zap.String("question", newDomainQuiz.Question),
					)

					// ---> START NEW CODE FOR QUIZ EVALUATION <---
					s.logger.Info("Attempting to generate and save QuizEvaluation", zap.String("quiz_id", newDomainQuiz.ID))

					existingEval, errEvalGet := s.quizRepo.GetQuizEvaluation(ctx, newDomainQuiz.ID)
					if errEvalGet != nil {
						s.logger.Error("Error checking for existing QuizEvaluation", zap.String("quiz_id", newDomainQuiz.ID), zap.Error(errEvalGet))
					} else if existingEval != nil {
						s.logger.Info("QuizEvaluation already exists for this quiz, skipping generation.", zap.String("quiz_id", newDomainQuiz.ID), zap.String("evaluation_id", existingEval.ID))
					} else {
						scoreRanges := []string{"0.8-1.0", "0.6-0.8", "0.3-0.6", "0-0.3"} // Default
						if s.cfg != nil && s.cfg.Batch.DefaultScoreRanges != nil && len(s.cfg.Batch.DefaultScoreRanges) > 0 {
							scoreRanges = s.cfg.Batch.DefaultScoreRanges
						}

						scoreEvalsDetails, errGenEvals := s.quizGenSvc.GenerateScoreEvaluationsForQuiz(ctx, &newDomainQuiz, scoreRanges)
						if errGenEvals != nil {
							s.logger.Error("Failed to generate score evaluations from LLM",
								zap.String("quiz_id", newDomainQuiz.ID),
								zap.Error(errGenEvals),
							)
						} else if len(scoreEvalsDetails) == 0 && len(scoreRanges) > 0 {
							s.logger.Warn("LLM returned no score evaluation details", zap.String("quiz_id", newDomainQuiz.ID))
						} else {
							quizEval := domain.NewQuizEvaluation(
								newDomainQuiz.ID,
								len(newDomainQuiz.Keywords),
								newDomainQuiz.Keywords,
								scoreRanges,
								newDomainQuiz.ModelAnswers,
								fmt.Sprintf("Automated rubric for quiz %s based on predefined score ranges.", newDomainQuiz.ID),
								scoreEvalsDetails,
							)

							if errVal := quizEval.Validate(); errVal != nil {
								s.logger.Error("Failed to validate QuizEvaluation object",
									zap.String("quiz_id", newDomainQuiz.ID),
									zap.Error(errVal),
								)
							} else {
								errSaveEval := s.quizRepo.SaveQuizEvaluation(ctx, quizEval) // Pass context
								if errSaveEval != nil {
									s.logger.Error("Failed to save QuizEvaluation",
										zap.String("quiz_id", newDomainQuiz.ID),
										zap.Error(errSaveEval),
									)
								} else {
									s.logger.Info("Successfully generated and saved QuizEvaluation",
										zap.String("quiz_id", newDomainQuiz.ID),
										zap.String("evaluation_id", quizEval.ID),
									)
								}
							}
						}
					}
					// ---> END NEW CODE FOR QUIZ EVALUATION <---
				}
			} else {
				s.logger.Info("Skipped saving quiz due to similarity",
					zap.String("question", generatedQuiz.Question), // Corrected field
				)
			}
		} // End loop for generatedQuizzesData
		s.logger.Info("Finished processing subcategory", zap.String("subcategory_id", subCategoryID))
	}

	s.logger.Info("Batch quiz generation process completed", zap.Time("end_time", time.Now()))
	return nil
}
