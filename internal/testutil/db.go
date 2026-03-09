package testutil

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/env"
)

// DBForTest returns a DB connection for tests. Skips if DB is unavailable.
func DBForTest(t *testing.T) *sql.DB {
	t.Helper()
	connStr := env.DatabaseURL()
	if connStr == "" {
		connStr = "postgres://postgres:gym-dev-2025@localhost:15432/postgres?sslmode=disable"
	}
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Skip("DB not available:", err)
	}
	if err := db.Ping(); err != nil {
		t.Skip("DB not reachable (proxy may be down):", err)
	}
	return db
}
