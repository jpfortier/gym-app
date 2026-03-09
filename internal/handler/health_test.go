package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHealth_ok(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	Health(db)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", got)
	}
	if body := rec.Body.String(); body != `{"status":"ok"}`+"\n" {
		t.Errorf("got body %q", rec.Body.String())
	}
}

func TestHealth_dbDown(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://postgres:wrong@localhost:9999/nonexistent?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	Health(db)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want 503", rec.Code)
	}
}
