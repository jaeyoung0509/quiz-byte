package dto

// CategoryResponse represents a category in the API response
// @Description Category information
type CategoryResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SubCategoryResponse represents a subcategory in the API response
type SubCategoryResponse struct {
	ID          int64  `json:"id"`
	CategoryID  int64  `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// QuizResponse represents a quiz in the API response
// @Description Quiz information
type QuizResponse struct {
	ID           int64    `json:"id"`
	Question     string   `json:"question"`
	ModelAnswers []string `json:"model_answers,omitempty"`
	Keywords     []string `json:"keywords"`
	DiffLevel    string   `json:"diff_level"`
}

// AnswerRequest represents a user's answer in the API request
// @Description Request body for checking an answer
type AnswerRequest struct {
	QuizID     int64  `json:"quiz_id"`
	UserAnswer string `json:"user_answer"`
}

// AnswerResponse represents the evaluation result in the API response
type AnswerResponse struct {
	Score          float64  `json:"score"`                  // 종합 점수 (0.0 ~ 1.0)
	Explanation    string   `json:"explanation"`            // LLM이 생성한 피드백
	KeywordMatches []string `json:"keyword_matches"`        // 매칭된 키워드들
	Completeness   float64  `json:"completeness"`           // 답변 완성도 (0.0 ~ 1.0)
	Relevance      float64  `json:"relevance"`              // 답변 관련성 (0.0 ~ 1.0)
	Accuracy       float64  `json:"accuracy"`               // 답변 정확도 (0.0 ~ 1.0)
	ModelAnswer    string   `json:"model_answer,omitempty"` // 모범 답안 (선택적)
	NextQuizID     int64    `json:"next_quiz_id,omitempty"` // 다음 문제 ID (유사도 기반)
}

// QuizEvaluationResponse represents the evaluation criteria in the API response
type QuizEvaluationResponse struct {
	ScoreRange  string `json:"score_range"`
	Explanation string `json:"explanation"`
}

// ErrorResponse represents an error in the API response
type ErrorResponse struct {
	Error string `json:"error"`
}
