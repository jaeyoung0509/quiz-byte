package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
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

// Category 모델 (변경 없음)
type Category struct {
	ID            int64          `gorm:"primaryKey;autoIncrement"`
	Name          string         `gorm:"size:100;not null;uniqueIndex"`
	Description   string         `gorm:"size:500"`
	CreatedAt     time.Time      `gorm:"not null"`
	UpdatedAt     time.Time      `gorm:"not null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	SubCategories []SubCategory  `gorm:"foreignKey:CategoryID"`
}

func (Category) TableName() string {
	return "categories"
}

// SubCategory 모델 (변경 없음)
type SubCategory struct {
	ID          int64          `gorm:"primaryKey;autoIncrement"`
	CategoryID  int64          `gorm:"not null;index"`
	Name        string         `gorm:"size:100;not null"`
	Description string         `gorm:"size:500"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Category    Category       `gorm:"foreignKey:CategoryID"`
	Quizzes     []Quiz         `gorm:"foreignKey:SubCategoryID"`
}

func (SubCategory) TableName() string {
	return "sub_categories"
}

// Quiz 모델 (GORM 태그는 이전과 동일하게 type:clob 유지)
type Quiz struct {
	ID            int64          `gorm:"primaryKey;autoIncrement"`
	Question      string         `gorm:"type:text;not null"`
	ModelAnswers  string         `gorm:"type:clob;not null"` // CLOB 타입 유지
	Keywords      string         `gorm:"type:clob;not null"` // CLOB 타입 유지
	Difficulty    int            `gorm:"not null"`
	SubCategoryID int64          `gorm:"not null;index"`
	CreatedAt     time.Time      `gorm:"not null"`
	UpdatedAt     time.Time      `gorm:"not null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	SubCategory   SubCategory    `gorm:"foreignKey:SubCategoryID"`
	Answers       []Answer       `gorm:"foreignKey:QuizID"`
}

func (Quiz) TableName() string {
	return "quizzes"
}

// Answer 모델 (GORM 태그는 이전과 동일하게 type:clob 유지)
type Answer struct {
	ID             int64          `gorm:"primaryKey;autoIncrement"`
	QuizID         int64          `gorm:"not null;index"`
	UserAnswer     string         `gorm:"type:text;not null"`
	Score          float64        `gorm:"type:decimal(3,2);not null"`
	Explanation    string         `gorm:"type:text;not null"`
	KeywordMatches StringSlice    `gorm:"type:clob;not null"` // CLOB 타입 유지
	Completeness   float64        `gorm:"type:decimal(3,2);not null"`
	Relevance      float64        `gorm:"type:decimal(3,2);not null"`
	Accuracy       float64        `gorm:"type:decimal(3,2);not null"`
	AnsweredAt     time.Time      `gorm:"not null"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (Answer) TableName() string {
	return "answers"
}
