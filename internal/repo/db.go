package repo

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Runner is the subset of *sql.DB and *sql.Tx that repos need. Methods that
// must participate in a caller-owned transaction take a Runner so the caller
// can pass either the DB (autocommit) or a Tx (transactional).
type Runner interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return db, nil
}

func nullInt64(v *int64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func scanNullInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

