package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestQueryHistory_returnsEntries(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "query-" + uuid.New().String(), Email: "q@test.com", Name: "Q"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press/standard:", err)
	}

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-20")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	logentryRepo := logentry.NewRepo(db)
	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench 135x8"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	queryService := query.NewService(exerciseRepo, logentryRepo, sessionRepo)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /query", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(QueryHistory(queryService, exerciseRepo))))

	req := httptest.NewRequest(http.MethodGet, "/query?category=bench+press&variant=standard", nil)
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
	entries, _ := out["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

func TestQueryHistory_missingCategoryReturns400(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "query-400-" + uuid.New().String(), Email: "q4@test.com", Name: "Q4"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	sessionRepo := session.NewRepo(db)
	queryService := query.NewService(exerciseRepo, logentryRepo, sessionRepo)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /query", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(QueryHistory(queryService, exerciseRepo))))

	req := httptest.NewRequest(http.MethodGet, "/query", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400", rec.Code)
	}
}
