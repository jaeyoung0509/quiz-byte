package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"
)

const stringDelimiter = "|||"

// StringSlice is a custom type for handling string arrays in GORM
type StringSlice []string

// Value implements the driver.Valuer interface
// 이 메서드는 StringSlice를 데이터베이스에 저장될 값으로 변환합니다.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil || len(s) == 0 {
		// nil 또는 빈 슬라이스는 빈 문자열로 저장 (테스트 케이스 "nil slice", "empty slice"와 일치)
		return "", nil
	}
	// 슬라이스의 요소들을 stringDelimiter로 합쳐서 하나의 문자열로 만듭니다.
	return strings.Join(s, stringDelimiter), nil
}

// Scan implements the sql.Scanner interface
// 이 메서드는 데이터베이스 값을 읽어 StringSlice로 변환합니다.
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{} // DB NULL은 빈 슬라이스로 (테스트 케이스 "nil input"과 일치)
		return nil
	}

	var strValue string
	switch v := value.(type) {
	case []byte:
		strValue = string(v)
	case string:
		strValue = v
	default:
		return errors.New("StringSlice Scan: unsupported type " + fmt.Sprintf("%T", value))
	}

	if strValue == "" {
		// DB의 빈 문자열은 빈 슬라이스로 (테스트 케이스 "empty string input"과 일치)
		// strings.Split("", stringDelimiter)는 [""]를 반환하므로,
		// 빈 문자열일 경우 명시적으로 빈 슬라이스를 할당합니다.
		*s = StringSlice{}
		return nil
	}

	// stringDelimiter를 기준으로 문자열을 분리하여 슬라이스로 만듭니다.
	*s = strings.Split(strValue, stringDelimiter)
	return nil
}

// Category 모델
type Category struct {
	ID          string     `db:"id"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (Category) TableName() string {
	return "categories"
}

// SubCategory 모델
type SubCategory struct {
	ID          string     `db:"id"`
	CategoryID  string     `db:"category_id"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (SubCategory) TableName() string {
	return "sub_categories"
}

// Quiz 모델
type Quiz struct {
	ID            string     `db:"id"`
	Question      string     `db:"question"`
	ModelAnswers  string     `db:"model_answers"`
	Keywords      string     `db:"keywords"`
	Difficulty    int        `db:"difficulty"`
	SubCategoryID string     `db:"sub_category_id"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}

func (Quiz) TableName() string {
	return "quizzes"
}

// Answer 모델
type Answer struct {
	ID             string      `db:"id"`
	QuizID         string      `db:"quiz_id"`
	UserAnswer     string      `db:"user_answer"`
	Score          float64     `db:"score"`
	Explanation    string      `db:"explanation"`
	KeywordMatches StringSlice `db:"keyword_matches"`
	Completeness   float64     `db:"completeness"`
	Relevance      float64     `db:"relevance"`
	Accuracy       float64     `db:"accuracy"`
	AnsweredAt     time.Time   `db:"answered_at"`
	CreatedAt      time.Time   `db:"created_at"`
	UpdatedAt      time.Time   `db:"updated_at"`
	DeletedAt      *time.Time  `db:"deleted_at"`
}

func (Answer) TableName() string {
	return "answers"
}

// QuizEvaluation 모델 (sqlx)
type QuizEvaluation struct {
	ID              string      `db:"id"`
	QuizID          string      `db:"quiz_id"`
	MinimumKeywords int         `db:"minimum_keywords"`
	RequiredTopics  StringSlice `db:"required_topics"` // StringSlice 사용 가정
	ScoreRanges     StringSlice `db:"score_ranges"`    // StringSlice 사용
	SampleAnswers   StringSlice `db:"sample_answers"`  // StringSlice 사용
	RubricDetails   string      `db:"rubric_details"`
	CreatedAt       time.Time   `db:"created_at"`
	UpdatedAt       time.Time   `db:"updated_at"`
	DeletedAt       *time.Time  `db:"deleted_at"`

	// 점수대별 상세 평가 정보를 JSON 문자열로 저장
	ScoreEvaluations string `db:"score_evaluations"` // JSON string representing []ScoreEvaluationDetail
}

func (QuizEvaluation) TableName() string {
	return "quiz_evaluations"
}
