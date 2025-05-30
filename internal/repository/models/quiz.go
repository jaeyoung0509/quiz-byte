package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// StringSlice is a custom type for handling string arrays in GORM
type StringSlice []string

// Value implements the driver.Valuer interface
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		// nil 슬라이스인 경우 DB에 빈 JSON 배열 문자열 "[]"로 저장
		return "[]", nil
	}
	// 실제 데이터가 있는 경우 JSON으로 마샬링 후 문자열로 변환
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(jsonData), nil // []byte 대신 string 반환
}

// Scan implements the sql.Scanner interface
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{} // DB NULL은 빈 슬라이스로
		return nil
	}

	var bytesToParse []byte

	switch v := value.(type) {
	case []byte:
		bytesToParse = v
	case string:
		bytesToParse = []byte(v)
	default:
		return errors.New("StringSlice Scan: unsupported type " + fmt.Sprintf("%T", value))
	}

	if len(bytesToParse) == 0 {
		*s = StringSlice{} // DB 빈 문자열은 빈 슬라이스로
		return nil
	}
	// DB에 "null" 문자열이 저장된 경우 (이전 로직 등으로 인해) 빈 슬라이스로 처리
	if string(bytesToParse) == "null" {
		*s = StringSlice{}
		return nil
	}

	return json.Unmarshal(bytesToParse, s)
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
