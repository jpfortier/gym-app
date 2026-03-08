package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

func dbForTest(t *testing.T) *sql.DB {
	t.Helper()
	connStr := os.Getenv("DATABASE_URL")
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

func TestSessionsList_returnsUserSessions(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "sessions-list-" + uuid.New().String(), Email: "sl@test.com", Name: "SL"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-16")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /sessions", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(SessionsList(sessionRepo))))

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Errorf("got %d sessions, want 1", len(out))
	}
	if len(out) >= 1 && out[0]["date"] != "2025-03-16" {
		t.Errorf("got date %v, want 2025-03-16", out[0]["date"])
	}
}

func TestSessionDetail_returnsSessionWithEntries(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createSessionsTestUser(t, db, ctx)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-17")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	logentryRepo := logentry.NewRepo(db)
	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variantID, RawSpeech: "bench 135x8"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	exerciseRepo := exercise.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /sessions/{id}", auth.RequireAuth(verifier, user.NewRepo(db), "aud")(http.HandlerFunc(SessionDetail(sessionRepo, logentryRepo, exerciseRepo))))

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sess.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["date"] != "2025-03-17" {
		t.Errorf("got date %v", out["date"])
	}
	entries, _ := out["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

func TestSessionDetail_otherUserReturns404(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u1 := createSessionsTestUser(t, db, ctx)
	u2 := createSessionsTestUser(t, db, ctx)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1, $2)", u1.ID, u2.ID)

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-18")
	sess := &session.Session{UserID: u1.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u2.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /sessions/{id}", auth.RequireAuth(verifier, user.NewRepo(db), "aud")(http.HandlerFunc(SessionDetail(sessionRepo, logentry.NewRepo(db), exercise.NewRepo(db)))))

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sess.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404", rec.Code)
	}
}

func createSessionsTestUser(t *testing.T, db *sql.DB, ctx context.Context) *user.User {
	t.Helper()
	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "sessions-" + uuid.New().String(), Email: "s@test.com", Name: "S"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	return u
}

type mockVerifier struct {
	payload *idtoken.Payload
}

func (m *mockVerifier) Verify(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
	return m.payload, nil
}
