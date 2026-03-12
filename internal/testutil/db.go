package testutil

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/jpfortier/gym-app/internal/env"
)

func init() {
	_ = godotenv.Load()
}

// DBForTest returns a DB connection for tests. Uses GYM_DATABASE_URL from .env.
// Connect to Fly Postgres via proxy: run `fly proxy 15432:5432 -a gym-app-pg` and set
// GYM_DATABASE_URL=postgres://postgres:YOUR_PASSWORD@localhost:15432/postgres?sslmode=disable
// Do not use local Postgres (port 5432); use Fly via proxy on 15432.
func DBForTest(t *testing.T) *sql.DB {
	t.Helper()
	connStr := env.DatabaseURL()
	if connStr == "" {
		t.Fatal("GYM_DATABASE_URL not set. Use Fly Postgres: fly proxy 15432:5432 -a gym-app-pg, then set GYM_DATABASE_URL in .env")
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
