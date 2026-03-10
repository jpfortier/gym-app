package testutil

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/env"
)

// DBForTest returns a DB connection for tests. Fails if DB is unavailable.
func DBForTest(t *testing.T) *sql.DB {
	t.Helper()
	connStr := env.DatabaseURL()
	if connStr == "" {
		connStr = "postgres://postgres:PASSWORD@prtracks.com:5432/postgres?sslmode=require"
	}
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatal("DB not available:", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatal("DB not reachable:", err)
	}
	return db
}
