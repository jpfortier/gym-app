package logentry

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestService_CreateLogEntry(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createLogEntryTestUser(t, db, ctx)
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) }()

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal("no seeded variant:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	repo := NewRepo(db)
	svc := NewService(repo, sessionSvc)

	w145 := 145.0
	entry, err := svc.CreateLogEntry(ctx, u.ID, "2025-03-13", variantID, "bench 145x6", "", []SetInput{
		{Weight: &w145, Reps: 6, SetOrder: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID == uuid.Nil {
		t.Error("expected ID set")
	}
	if entry.SessionID == uuid.Nil {
		t.Error("expected SessionID set")
	}

	got, err := repo.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Sets) != 1 || got.RawSpeech != "bench 145x6" {
		t.Errorf("got %+v", got)
	}
}

func TestService_CreateLogEntry_sameDayReusesSession(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createLogEntryTestUser(t, db, ctx)
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) }()

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal("no seeded variant:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	repo := NewRepo(db)
	svc := NewService(repo, sessionSvc)

	w1, w2 := 135.0, 140.0
	e1, err := svc.CreateLogEntry(ctx, u.ID, "2025-03-14", variantID, "bench 135x8", "", []SetInput{{Weight: &w1, Reps: 8, SetOrder: 1}})
	if err != nil {
		t.Fatal(err)
	}
	e2, err := svc.CreateLogEntry(ctx, u.ID, "2025-03-14", variantID, "squat 140x5", "", []SetInput{{Weight: &w2, Reps: 5, SetOrder: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if e1.SessionID != e2.SessionID {
		t.Errorf("expected same session for same day: %s vs %s", e1.SessionID, e2.SessionID)
	}
}

func TestService_CreateLogEntry_sessionAndLogEntryIntegration(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createLogEntryTestUser(t, db, ctx)
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) }()

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal("no seeded variant:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logRepo := NewRepo(db)
	logSvc := NewService(logRepo, sessionSvc)

	w := 185.0
	entry, err := logSvc.CreateLogEntry(ctx, u.ID, "2025-03-15", variantID, "deadlift 185x3", "", []SetInput{{Weight: &w, Reps: 3, SetOrder: 1}})
	if err != nil {
		t.Fatal(err)
	}

	sess, _ := sessionSvc.GetOrCreateForDate(ctx, u.ID, "2025-03-15")
	if sess == nil || sess.ID != entry.SessionID {
		t.Errorf("session from service should match entry.SessionID: sess=%+v entry.SessionID=%s", sess, entry.SessionID)
	}
	list, _ := logRepo.ListBySession(ctx, entry.SessionID)
	if len(list) != 1 || list[0].ID != entry.ID {
		t.Errorf("ListBySession should return created entry: got %+v", list)
	}
}

func createLogEntryTestUser(t *testing.T, db *sql.DB, ctx context.Context) *user.User {
	t.Helper()
	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "logsvc-" + uuid.New().String(), Email: "logsvc-" + uuid.New().String() + "@test.com", Name: "Log Svc"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	return u
}
