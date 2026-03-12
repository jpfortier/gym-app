package command

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
	"github.com/jpfortier/gym-app/internal/workoutcontext"
	"github.com/jpfortier/gym-app/internal/testutil"
)

func dbForTest(t *testing.T) *sql.DB { return testutil.DBForTest(t) }

func TestExecutor_EnsureSession(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-es")

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	exec := NewExecutor(sessionSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	wc := &workoutcontext.WorkoutContext{RefObjects: workoutcontext.RefObjects{}}
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: EnsureSession, Date: "2025-03-25"},
	}, wc, "2025-03-24")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	sess, err := sessionRepo.GetByUserAndDate(ctx, u.ID, "2025-03-25")
	if err != nil || sess == nil {
		t.Fatalf("expected session: %v", err)
	}
}

func TestExecutor_CreateExerciseEntry(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-ce")

	exerciseRepo := exercise.NewRepo(db)
	exerciseSvc := exercise.NewService(exerciseRepo, nil)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentryRepo := logentry.NewRepo(db)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)
	prSvc := pr.NewService(pr.NewRepo(db))

	exec := NewExecutor(sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo, nil, nil, nil, prSvc)

	w := 135.0
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: EnsureSession, Date: "2025-03-26"},
		{Type: CreateExerciseEntry, Exercise: "bench press", Variant: "standard", RawSpeech: "bench 135x8", Sets: []SetSpec{{Weight: &w, Reps: 8}}},
	}, nil, "2025-03-26")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if len(result.CreatedEntryIDs) != 1 {
		t.Errorf("expected 1 created entry, got %d", len(result.CreatedEntryIDs))
	}
	entries, _ := logentryRepo.ListByUserAndVariantWithDateRange(ctx, u.ID, variant.ID, "2025-03-26", "2025-03-26", 5)
	if len(entries) != 1 || len(entries[0].Sets) != 1 {
		t.Errorf("expected 1 entry with 1 set, got %d entries", len(entries))
		if len(entries) > 0 {
			t.Errorf("  sets: %d", len(entries[0].Sets))
		}
	}
}

func TestExecutor_AppendSet(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-as")

	exerciseRepo := exercise.NewRepo(db)
	exerciseSvc := exercise.NewService(exerciseRepo, nil)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "deadlift", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded deadlift:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentryRepo := logentry.NewRepo(db)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)
	prSvc := pr.NewService(pr.NewRepo(db))

	parsed, _ := time.Parse("2006-01-02", "2025-03-27")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w1 := 185.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "dl 185x5"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w1, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full, _ := logentryRepo.GetByID(ctx, entry.ID)
	if full == nil || len(full.Sets) == 0 {
		t.Fatal("expected entry with sets")
	}
	setID := full.Sets[0].ID.String()

	exec := NewExecutor(sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo, nil, nil, nil, prSvc)
	wc := &workoutcontext.WorkoutContext{
		RefObjects: workoutcontext.RefObjects{
			LastCreatedSet: setID,
			LastExercise:   "deadlift standard",
			LastSession:    "2025-03-27",
		},
	}
	w2 := 205.0
	reps := 3
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: AppendSet, TargetRef: "last_created_set", Weight: &w2, Reps: &reps},
	}, wc, "2025-03-27")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	updated, _ := logentryRepo.GetByID(ctx, entry.ID)
	if len(updated.Sets) != 2 {
		t.Errorf("expected 2 sets, got %d", len(updated.Sets))
	}
	if len(updated.Sets) >= 2 && (updated.Sets[1].Weight == nil || *updated.Sets[1].Weight != 205) {
		t.Errorf("expected second set 205x3, got %v", updated.Sets[1])
	}
	if len(result.PRs) == 0 {
		t.Error("expected PR when appending heavier set (205x3 beats 185x5)")
	}
}

func TestExecutor_UpdateSet(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-us")

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "squat", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded squat:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentryRepo := logentry.NewRepo(db)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)

	parsed, _ := time.Parse("2006-01-02", "2025-03-28")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full, _ := logentryRepo.GetByID(ctx, entry.ID)
	if full == nil || len(full.Sets) == 0 {
		t.Fatal("expected entry with sets")
	}
	setID := full.Sets[0].ID.String()

	exec := NewExecutor(sessionSvc, logentrySvc, logentryRepo, exercise.NewService(exerciseRepo, nil), exerciseRepo, nil, nil, nil, nil)
	wc := &workoutcontext.WorkoutContext{RefObjects: workoutcontext.RefObjects{LastCreatedSet: setID}}
	newW := 145.0
	newR := 6
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: UpdateSet, TargetRef: "last_created_set", Changes: &SetChanges{Weight: &newW, Reps: &newR}},
	}, wc, "2025-03-28")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	updated, _ := logentryRepo.GetByID(ctx, entry.ID)
	if updated.Sets[0].Weight == nil || *updated.Sets[0].Weight != 145 || updated.Sets[0].Reps != 6 {
		t.Errorf("expected 145x6, got %v x %d", updated.Sets[0].Weight, updated.Sets[0].Reps)
	}
}

func TestExecutor_DeleteSet(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-ds")

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentryRepo := logentry.NewRepo(db)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)

	parsed, _ := time.Parse("2006-01-02", "2025-03-29")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	full, _ := logentryRepo.GetByID(ctx, entry.ID)
	if full == nil || len(full.Sets) == 0 {
		t.Fatal("expected entry with sets")
	}
	setID := full.Sets[0].ID.String()

	exec := NewExecutor(sessionSvc, logentrySvc, logentryRepo, exercise.NewService(exerciseRepo, nil), exerciseRepo, nil, nil, nil, nil)
	wc := &workoutcontext.WorkoutContext{RefObjects: workoutcontext.RefObjects{LastCreatedSet: setID}}
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: DeleteSet, TargetRef: "last_created_set"},
	}, wc, "2025-03-29")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	updated, _ := logentryRepo.GetByID(ctx, entry.ID)
	if len(updated.Sets) != 0 {
		t.Errorf("expected 0 sets after delete, got %d", len(updated.Sets))
	}
}

func TestExecutor_RestoreEntry(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-re")
	userRepo := user.NewRepo(db)

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "deadlift", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded deadlift:", err)
	}

	sessionRepo := session.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentryRepo := logentry.NewRepo(db)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)

	parsed, _ := time.Parse("2006-01-02", "2025-03-30")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w := 185.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "dl"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}
	ok, err := logentryRepo.DisableEntry(ctx, entry.ID, u.ID)
	if err != nil || !ok {
		t.Fatal("disable failed:", err)
	}

	exec := NewExecutor(sessionSvc, logentrySvc, logentryRepo, exercise.NewService(exerciseRepo, nil), exerciseRepo, userRepo, nil, nil, nil)
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: RestoreEntry},
	}, nil, "2025-03-30")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	restored, _ := logentryRepo.GetByID(ctx, entry.ID)
	if restored == nil || restored.DisabledAt != nil {
		t.Error("expected entry to be restored (disabled_at nil)")
	}
}

func TestExecutor_UpdateName(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-un")
	userRepo := user.NewRepo(db)

	exec := NewExecutor(nil, nil, nil, nil, nil, userRepo, nil, nil, nil)
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: UpdateName, Name: "JP"},
	}, nil, "")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if result.StoredName != "JP" {
		t.Errorf("expected StoredName JP, got %q", result.StoredName)
	}
	updated, _ := userRepo.GetByGoogleID(ctx, u.GoogleID)
	if updated.Name != "JP" {
		t.Errorf("expected user name JP, got %q", updated.Name)
	}
}

func TestExecutor_SetName(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-sn")
	userRepo := user.NewRepo(db)

	throttle := ai.NewThrottler(60, 1000, 10)
	aiClient := ai.NewClient(throttle, nil)
	nameHandler := name.NewHandler(aiClient)

	exec := NewExecutor(nil, nil, nil, nil, nil, userRepo, nameHandler, nil, nil)
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: SetName, Name: "Marty"},
	}, nil, "")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	if result.StoredName == "" {
		t.Error("expected StoredName to be set")
	}
}

func TestExecutor_CreateNote(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "cmd-cn")

	exerciseRepo := exercise.NewRepo(db)
	exerciseSvc := exercise.NewService(exerciseRepo, nil)
	notesRepo := notes.NewRepo(db)

	exec := NewExecutor(nil, nil, nil, exerciseSvc, exerciseRepo, nil, nil, notesRepo, nil)
	result := exec.Execute(ctx, u.ID, []Command{
		{Type: CreateNote, Category: "deadlift", Variant: "rdl", Content: "warm up hamstrings first"},
	}, nil, "")
	if !result.Success {
		t.Fatalf("expected success: %s", result.Error)
	}
	notesList, _ := notesRepo.ListByUser(ctx, u.ID, 10)
	if len(notesList) != 1 || notesList[0].Content != "warm up hamstrings first" {
		t.Errorf("expected 1 note, got %d: %v", len(notesList), notesList)
	}
}
