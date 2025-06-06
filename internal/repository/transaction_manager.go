package repository

import (
	"context"
	"fmt"

	"quiz-byte/internal/domain"

	"github.com/jmoiron/sqlx"
)

// contextKey는 context value의 key 타입
type contextKey string

const (
	// TransactionContextKey 트랜잭션을 저장하는 context key
	TransactionContextKey contextKey = "tx"
)

// GetExecutor context에서 트랜잭션을 가져오거나 기본 DB를 반환하는 헬퍼 함수
func GetExecutor(ctx context.Context, db DBTX) DBTX {
	if tx := ctx.Value(TransactionContextKey); tx != nil {
		if sqlxTx, ok := tx.(*sqlx.Tx); ok {
			return sqlxTx
		}
	}
	return db
}

// TransactionManagerAdapter sqlx.DB를 사용한 트랜잭션 매니저 구현체
type TransactionManagerAdapter struct {
	db *sqlx.DB
}

// NewTransactionManagerAdapter 새로운 트랜잭션 매니저 어댑터 인스턴스를 생성
func NewTransactionManagerAdapter(db *sqlx.DB) domain.TransactionManager {
	return &TransactionManagerAdapter{db: db}
}

// WithTransaction 트랜잭션 내에서 함수를 실행 (도메인 인터페이스 구현)
func (tma *TransactionManagerAdapter) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tma.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				// 롤백 실패는 로그로만 기록
				fmt.Printf("failed to rollback transaction: %v\n", rollbackErr)
			}
			panic(p) // 원래 패닉을 다시 발생
		}
	}()

	// 트랜잭션 컨텍스트를 만들어서 전달
	txCtx := context.WithValue(ctx, TransactionContextKey, tx)

	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction: %v (original error: %w)", rollbackErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
