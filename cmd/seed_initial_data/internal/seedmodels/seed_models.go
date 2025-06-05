package seedmodels

// SeedQuiz defines the structure for a quiz item in the JSON seed file.
type SeedQuiz struct {
	Question     string   `json:"question"`
	ModelAnswers []string `json:"model_answers"`
	Keywords     []string `json:"keywords"`
	Difficulty   string   `json:"difficulty"`
}

// SeedSubCategory defines the structure for a sub-category in the JSON seed file.
type SeedSubCategory struct {
	Name        string     `json:"sub_category_name"`
	Description string     `json:"sub_category_description"`
	Quizzes     []SeedQuiz `json:"quizzes"`
}

// SeedCategory defines the structure for a category in the JSON seed file.
type SeedCategory struct {
	Name        string            `json:"category_name"`
	Description string            `json:"category_description"`
	SubCategories []SeedSubCategory `json:"sub_categories"`
}
