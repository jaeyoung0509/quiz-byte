package repository

import (
	"context"
	"database/sql" // Required for sql.Result
	"github.com/jmoiron/sqlx"
)

// DBTX is an interface abstracting *sqlx.DB and *sqlx.Tx for repository use.
type DBTX interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	NamedQueryContext(ctx context.Context, query string, arg interface{}) (*sqlx.Rows, error)
}
