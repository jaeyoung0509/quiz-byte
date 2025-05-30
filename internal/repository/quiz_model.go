package repository

import (
	"database/sql"
	"time"
)

// Quiz represents the quiz data model.
type Quiz struct {
	// id (PK, NUMBER, 자동 증가 또는 UUID)
	ID uint `gorm:"primaryKey"` // Using uint for auto-incrementing primary key

	// main_category (VARCHAR2, 예: "language", "network", "os", "system_design", "database") - 대분류
	MainCategory string `gorm:"type:VARCHAR2(255);not null"`

	// sub_category (VARCHAR2, 예: "python", "go", "tcp_ip", "linux", "microservices") - 중분류
	SubCategory string `gorm:"type:VARCHAR2(255);not null"`

	// question_text (CLOB 또는 VARCHAR2(4000)) - 질문 내용
	QuestionText string `gorm:"type:CLOB;not null"` // Using CLOB for potentially long text

	// answer_options (JSON 또는 CLOB, 객관식 보기 등) - 선택적, 문제 유형에 따라
	AnswerOptions sql.NullString `gorm:"type:CLOB"` // Using sql.NullString for optional CLOB or JSON

	// correct_answer (CLOB 또는 VARCHAR2(4000)) - 정답
	CorrectAnswer string `gorm:"type:CLOB;not null"`

	// explanation (CLOB 또는 VARCHAR2(4000)) - 정답 해설
	Explanation sql.NullString `gorm:"type:CLOB"` // Using sql.NullString for optional CLOB

	// difficulty (NUMBER 또는 VARCHAR2, 예: 1-5점 또는 "상", "중", "하") - 난이도
	Difficulty string `gorm:"type:VARCHAR2(50);not null"` // Using VARCHAR2 for flexibility (e.g., "상", "중", "하" or numeric string)

	// correlation_coefficient (JSON 또는 별도 테이블) - 다른 질문과의 상관계수 (구체적 구조 설계 필요: related_question_id, coefficient_value, type 등)
	// For simplicity, using CLOB for JSON, can be expanded to a separate table later if needed.
	CorrelationCoefficient sql.NullString `gorm:"type:CLOB"` // Using sql.NullString for optional CLOB for JSON

	// created_at (TIMESTAMP WITH TIME ZONE)
	CreatedAt time.Time `gorm:"type:TIMESTAMP WITH TIME ZONE;not null"`

	// updated_at (TIMESTAMP WITH TIME ZONE)
	UpdatedAt time.Time `gorm:"type:TIMESTAMP WITH TIME ZONE;not null"`

	// (선택) source (VARCHAR2) - 출제자/출처
	Source sql.NullString `gorm:"type:VARCHAR2(255)"` // Using sql.NullString for optional VARCHAR2

	// (선택) tags (JSON 또는 VARCHAR2, 콤마로 구분된 태그)
	// Using CLOB for JSON or comma-separated string.
	Tags sql.NullString `gorm:"type:CLOB"` // Using sql.NullString for optional CLOB for JSON or string

	// GORM fields for soft delete if needed in the future
	// DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName specifies the table name for the Quiz model.
func (Quiz) TableName() string {
	return "quizzes" // Or whatever table name you prefer
}
