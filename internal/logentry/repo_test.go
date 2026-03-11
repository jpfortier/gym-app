package logentry

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/testutil"
	"github.com/jpfortier/gym-app/internal/user"
)

func dbForTest(t *testing.T) *sql.DB { return testutil.DBForTest(t) }

func seedSessionAndVariant(t *testing.T, db *sql.DB, ctx context.Context) (sessID, variantID uuid.UUID) {
	t.Helper()
	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "logentry-" + uuid.New().String(), Email: "le-" + uuid.New().String() + "@test.com", Name: "LE"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-12")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	var vID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&vID); err != nil {
		t.Fatal("no seeded variant:", err)
	}
	return sess.ID, vID
}

func TestRepo_Create_GetByID(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	sessID, variantID := seedSessionAndVariant(t, db, ctx)

	repo := NewRepo(db)
	w140 := 140.0
	entry := &LogEntry{SessionID: sessID, ExerciseVariantID: variantID, RawSpeech: "bench 140x8"}
	sets := []SetInput{
		{Weight: &w140, Reps: 8, SetOrder: 1, SetType: "working"},
		{Weight: &w140, Reps: 8, SetOrder: 2, SetType: "working"},
	}
	if err := repo.Create(ctx, entry, sets); err != nil {
		t.Fatal(err)
	}
	if entry.ID == uuid.Nil {
		t.Error("expected ID set")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt set")
	}

	got, err := repo.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected entry")
	}
	if got.SessionID != sessID || got.ExerciseVariantID != variantID || got.RawSpeech != "bench 140x8" {
		t.Errorf("got %+v", got)
	}
	if len(got.Sets) != 2 {
		t.Errorf("got %d sets, want 2", len(got.Sets))
	}
	if len(got.Sets) >= 1 && (got.Sets[0].Weight == nil || *got.Sets[0].Weight != 140) {
		t.Errorf("set 0 weight: got %v, want 140", got.Sets[0].Weight)
	}
}

func TestRepo_ListBySession(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	sessID, variantID := seedSessionAndVariant(t, db, ctx)

	repo := NewRepo(db)
	w135 := 135.0
	entry := &LogEntry{SessionID: sessID, ExerciseVariantID: variantID}
	if err := repo.Create(ctx, entry, []SetInput{{Weight: &w135, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListBySession(ctx, sessID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("got %d entries, want 1", len(list))
	}
	if len(list) >= 1 && len(list[0].Sets) != 1 {
		t.Errorf("got %d sets, want 1", len(list[0].Sets))
	}
}

func TestRepo_Create_bodyweight(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	sessID, variantID := seedSessionAndVariant(t, db, ctx)

	repo := NewRepo(db)
	entry := &LogEntry{SessionID: sessID, ExerciseVariantID: variantID, RawSpeech: "10 pushups"}
	if err := repo.Create(ctx, entry, []SetInput{{Weight: nil, Reps: 10, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Sets) != 1 || got.Sets[0].Weight != nil {
		t.Errorf("bodyweight set should have nil weight: got %+v", got)
	}
}

func TestRepo_AppendSet(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	sessID, variantID := seedSessionAndVariant(t, db, ctx)

	repo := NewRepo(db)
	w := 135.0
	entry := &LogEntry{SessionID: sessID, ExerciseVariantID: variantID, RawSpeech: "bench"}
	if err := repo.Create(ctx, entry, []SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	w2 := 145.0
	setID, err := repo.AppendSet(ctx, entry.ID, &w2, 6, "working")
	if err != nil {
		t.Fatal(err)
	}
	if setID == uuid.Nil {
		t.Error("expected non-nil set ID")
	}
	got, _ := repo.GetByID(ctx, entry.ID)
	if len(got.Sets) != 2 {
		t.Fatalf("expected 2 sets, got %d", len(got.Sets))
	}
	if got.Sets[1].Weight == nil || *got.Sets[1].Weight != 145 || got.Sets[1].Reps != 6 {
		t.Errorf("expected second set 145x6, got %v x %d", got.Sets[1].Weight, got.Sets[1].Reps)
	}
}

func TestRepo_DeleteSet(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	sessID, variantID := seedSessionAndVariant(t, db, ctx)

	repo := NewRepo(db)
	w := 135.0
	entry := &LogEntry{SessionID: sessID, ExerciseVariantID: variantID, RawSpeech: "bench"}
	if err := repo.Create(ctx, entry, []SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(ctx, entry.ID)
	if len(got.Sets) == 0 {
		t.Fatal("expected sets")
	}
	setID := got.Sets[0].ID
	if err := repo.DeleteSet(ctx, setID); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.GetByID(ctx, entry.ID)
	if len(after.Sets) != 0 {
		t.Errorf("expected 0 sets after delete, got %d", len(after.Sets))
	}
}

func TestRepo_GetEntryIDBySetID(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	sessID, variantID := seedSessionAndVariant(t, db, ctx)

	repo := NewRepo(db)
	w := 135.0
	entry := &LogEntry{SessionID: sessID, ExerciseVariantID: variantID, RawSpeech: "bench"}
	if err := repo.Create(ctx, entry, []SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(ctx, entry.ID)
	if len(got.Sets) == 0 {
		t.Fatal("expected sets")
	}
	eid, err := repo.GetEntryIDBySetID(ctx, got.Sets[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if eid != entry.ID {
		t.Errorf("expected entry ID %s, got %s", entry.ID, eid)
	}
}
