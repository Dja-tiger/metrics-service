package repository

import (
	"database/sql"

	_ "github.com/lib/pq"
)

func NewPostgresDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	return sql.Open("postgres", dsn)
}
