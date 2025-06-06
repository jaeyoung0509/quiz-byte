package models

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"
)

const stringDelimiter = "|||"

// StringSlice implements custom serialization for []string to a single string for DB storage.
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
	ID          string       `db:"ID"`
	Name        string       `db:"NAME"`
	Description string       `db:"DESCRIPTION"`
	CreatedAt   time.Time    `db:"CREATED_AT"`
	UpdatedAt   time.Time    `db:"UPDATED_AT"`
	DeletedAt   sql.NullTime `db:"DELETED_AT"`
}

// SubCategory 모델
type SubCategory struct {
	ID          string       `db:"ID"`
	CategoryID  string       `db:"CATEGORY_ID"`
	Name        string       `db:"NAME"`
	Description string       `db:"DESCRIPTION"`
	CreatedAt   time.Time    `db:"CREATED_AT"`
	UpdatedAt   time.Time    `db:"UPDATED_AT"`
	DeletedAt   sql.NullTime `db:"DELETED_AT"`
}

// Quiz 모델
type Quiz struct {
	ID            string       `db:"ID"`
	Question      string       `db:"QUESTION"`
	ModelAnswers  string       `db:"MODEL_ANSWERS"`
	Keywords      string       `db:"KEYWORDS"`
	Difficulty    int          `db:"DIFFICULTY"`
	SubCategoryID string       `db:"SUB_CATEGORY_ID"`
	CreatedAt     time.Time    `db:"CREATED_AT"`
	UpdatedAt     time.Time    `db:"UPDATED_AT"`
	DeletedAt     sql.NullTime `db:"DELETED_AT"`
}

// Answer 모델
type Answer struct {
	ID             string       `db:"ID"`
	QuizID         string       `db:"QUIZ_ID"`
	UserAnswer     string       `db:"USER_ANSWER"`
	Score          float64      `db:"SCORE"`
	Explanation    string       `db:"EXPLANATION"`
	KeywordMatches StringSlice  `db:"KEYWORD_MATCHES"`
	Completeness   float64      `db:"COMPLETENESS"`
	Relevance      float64      `db:"RELEVANCE"`
	Accuracy       float64      `db:"ACCURACY"`
	AnsweredAt     time.Time    `db:"ANSWERED_AT"`
	CreatedAt      time.Time    `db:"CREATED_AT"`
	UpdatedAt      time.Time    `db:"UPDATED_AT"`
	DeletedAt      sql.NullTime `db:"DELETED_AT"`
}

// QuizEvaluation 모델 (sqlx)
type QuizEvaluation struct {
	ID              string         `db:"ID"`
	QuizID          string         `db:"QUIZ_ID"`
	MinimumKeywords int            `db:"MINIMUM_KEYWORDS"`
	RequiredTopics  sql.NullString `db:"REQUIRED_TOPICS"` // NULL 허용
	ScoreRanges     sql.NullString `db:"SCORE_RANGES"`    // NULL 허용
	SampleAnswers   sql.NullString `db:"SAMPLE_ANSWERS"`  // NULL 허용
	RubricDetails   sql.NullString `db:"RUBRIC_DETAILS"`  // NULL 허용
	CreatedAt       time.Time      `db:"CREATED_AT"`
	UpdatedAt       time.Time      `db:"UPDATED_AT"`
	DeletedAt       sql.NullTime   `db:"DELETED_AT"`
	// 점수대별 상세 평가 정보를 JSON 문자열로 저장
	ScoreEvaluations sql.NullString `db:"SCORE_EVALUATIONS"` // JSON string representing []ScoreEvaluationDetail, NULL 허용
}
