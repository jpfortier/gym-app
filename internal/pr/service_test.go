package pr

import (
	"context"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/testutil"
)

func TestCheckAndCreatePRs_firstEntryNoCelebration(t *testing.T) {
	db := testutil.DBForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "pr-first")

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press:", err)
	}

	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	prRepo := NewRepo(db)
	prSvc := NewService(prRepo)

	parsed, _ := time.Parse("2006-01-02", "2025-03-26")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench 135x8"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full, err := logentryRepo.GetByID(ctx, entry.ID)
	if err != nil || full == nil || len(full.Sets) == 0 {
		t.Fatal("expected entry with sets:", err)
	}

	created, err := prSvc.CheckAndCreatePRs(ctx, u.ID, full)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 {
		t.Errorf("first entry should not celebrate PR, got %d created", len(created))
	}

	all, _ := prRepo.ListByUser(ctx, u.ID)
	if len(all) != 1 {
		t.Errorf("first entry should store baseline PR in DB, got %d", len(all))
	}
}

func TestCheckAndCreatePRs_secondEntryBetterCelebrates(t *testing.T) {
	db := testutil.DBForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "pr-second")

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press:", err)
	}

	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	prRepo := NewRepo(db)
	prSvc := NewService(prRepo)

	parsed, _ := time.Parse("2006-01-02", "2025-03-26")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	w1 := 135.0
	entry1 := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench 135x8"}
	if err := logentryRepo.Create(ctx, entry1, []logentry.SetInput{{Weight: &w1, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full1, _ := logentryRepo.GetByID(ctx, entry1.ID)
	if full1 == nil {
		t.Fatal("expected entry1")
	}

	created1, err := prSvc.CheckAndCreatePRs(ctx, u.ID, full1)
	if err != nil {
		t.Fatal(err)
	}
	if len(created1) != 0 {
		t.Errorf("first entry should not celebrate, got %d", len(created1))
	}

	w2 := 140.0
	entry2 := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench 140x8"}
	if err := logentryRepo.Create(ctx, entry2, []logentry.SetInput{{Weight: &w2, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full2, _ := logentryRepo.GetByID(ctx, entry2.ID)
	if full2 == nil {
		t.Fatal("expected entry2")
	}

	created2, err := prSvc.CheckAndCreatePRs(ctx, u.ID, full2)
	if err != nil {
		t.Fatal(err)
	}
	if len(created2) != 1 {
		t.Errorf("second entry (better) should celebrate PR, got %d", len(created2))
	}
	if created2[0].Weight != 140 || (created2[0].Reps == nil || *created2[0].Reps != 8) {
		t.Errorf("expected 140x8 PR, got %.0fx%v", created2[0].Weight, created2[0].Reps)
	}
}

func TestCheckAndCreatePRs_secondEntryWorseNoCelebration(t *testing.T) {
	db := testutil.DBForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "pr-worse")

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "squat", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded squat:", err)
	}

	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	prRepo := NewRepo(db)
	prSvc := NewService(prRepo)

	parsed, _ := time.Parse("2006-01-02", "2025-03-26")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	w1 := 225.0
	entry1 := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat 225x1"}
	if err := logentryRepo.Create(ctx, entry1, []logentry.SetInput{{Weight: &w1, Reps: 1, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full1, _ := logentryRepo.GetByID(ctx, entry1.ID)
	if full1 == nil {
		t.Fatal("expected entry1")
	}

	created1, _ := prSvc.CheckAndCreatePRs(ctx, u.ID, full1)
	if len(created1) != 0 {
		t.Errorf("first 1RM should not celebrate, got %d", len(created1))
	}

	w2 := 200.0
	entry2 := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat 200x1"}
	if err := logentryRepo.Create(ctx, entry2, []logentry.SetInput{{Weight: &w2, Reps: 1, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full2, _ := logentryRepo.GetByID(ctx, entry2.ID)
	if full2 == nil {
		t.Fatal("expected entry2")
	}

	created2, err := prSvc.CheckAndCreatePRs(ctx, u.ID, full2)
	if err != nil {
		t.Fatal(err)
	}
	if len(created2) != 0 {
		t.Errorf("second entry (worse) should not celebrate, got %d", len(created2))
	}
}
