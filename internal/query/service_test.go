package query

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

func dbForTest(t *testing.T) *sql.DB {
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

func TestService_History_returnsEntries(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "query-svc-" + uuid.New().String(), Email: "qs@test.com", Name: "QS"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "deadlift", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded deadlift/standard:", err)
	}

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	logentryRepo := logentry.NewRepo(db)
	w := 185.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "deadlift 185x5"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)

	entries, variant, err := svc.History(ctx, u.ID, "deadlift", "standard", "", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if variant == nil {
		t.Fatal("expected variant from seeded deadlift/standard")
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
	if len(entries) >= 1 {
		if entries[0].SessionDate != "2025-03-21" {
			t.Errorf("got session_date %q, want 2025-03-21", entries[0].SessionDate)
		}
		if entries[0].RawSpeech != "deadlift 185x5" {
			t.Errorf("got raw_speech %q", entries[0].RawSpeech)
		}
		if len(entries[0].Sets) != 1 {
			t.Errorf("got %d sets, want 1", len(entries[0].Sets))
		}
	}
}

func TestService_History_notFoundReturnsNil(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := &user.User{GoogleID: "query-nf-" + uuid.New().String(), Email: "qn@test.com", Name: "QN"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	sessionRepo := session.NewRepo(db)
	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)

	entries, variant, err := svc.History(ctx, u.ID, "nonexistent exercise", "standard", "", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if variant != nil {
		t.Errorf("expected nil variant for nonexistent exercise")
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %d", len(entries))
	}
}
