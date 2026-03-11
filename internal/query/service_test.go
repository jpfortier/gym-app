package query

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/testutil"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

func dbForTest(t *testing.T) *sql.DB { return testutil.DBForTest(t) }

func TestService_History_returnsEntries(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "query-svc-" + uuid.New().String(), Email: "qs-" + uuid.New().String() + "@test.com", Name: "QS"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

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

	u := &user.User{GoogleID: "query-nf-" + uuid.New().String(), Email: "qn-" + uuid.New().String() + "@test.com", Name: "QN"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

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

func TestService_Query_emptyCategoryReturnsError(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-ec-" + uuid.New().String(), Email: "qec-" + uuid.New().String() + "@test.com", Name: "QEC"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	svc := NewService(exercise.NewRepo(db), logentry.NewRepo(db), session.NewRepo(db))
	_, err := svc.Query(ctx, u.ID, QueryParams{Category: ""})
	if err == nil {
		t.Error("expected error for empty category")
	}
}

func TestService_Query_nonexistentExerciseReturnsNil(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-ne-" + uuid.New().String(), Email: "qne-" + uuid.New().String() + "@test.com", Name: "QNE"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	svc := NewService(exercise.NewRepo(db), logentry.NewRepo(db), session.NewRepo(db))
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "nonexistent", Variant: "standard"})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Errorf("expected nil result for nonexistent exercise, got %+v", res)
	}
}

func TestService_Query_scopeMostRecent(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-smr-" + uuid.New().String(), Email: "qsmr-" + uuid.New().String() + "@test.com", Name: "QSMR"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "deadlift", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded deadlift/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)

	for _, d := range []string{"2025-03-20", "2025-03-21", "2025-03-22"} {
		parsed, _ := time.Parse("2006-01-02", d)
		sess := &session.Session{UserID: u.ID, Date: parsed}
		if err := sessionRepo.Create(ctx, sess); err != nil {
			t.Fatal(err)
		}
		w := 100.0
		if d == "2025-03-22" {
			w = 200.0
		}
		entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "dl " + d}
		if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "deadlift", Variant: "standard", Scope: "most_recent", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if len(res.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(res.Entries))
	}
	if res.Entries[0].SessionDate != "2025-03-22" {
		t.Errorf("got session_date %q, want 2025-03-22 (most recent)", res.Entries[0].SessionDate)
	}
	if len(res.Entries[0].Sets) != 1 || res.Entries[0].Sets[0].Weight == nil || *res.Entries[0].Sets[0].Weight != 200 {
		t.Errorf("expected 200 lb set, got %+v", res.Entries[0].Sets)
	}
}

func TestService_Query_scopeBest(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-sb-" + uuid.New().String(), Email: "qsb-" + uuid.New().String() + "@test.com", Name: "QSB"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)

	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	for _, w := range []float64{135, 185, 225} {
		entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench"}
		if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "bench press", Variant: "standard", Scope: "best", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(res.Entries))
	}
	if res.Entries[0].Sets[0].Weight == nil || *res.Entries[0].Sets[0].Weight != 225 {
		t.Errorf("expected best set 225, got %v", res.Entries[0].Sets[0].Weight)
	}
}

func TestService_Query_scopeAggregate(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-sa-" + uuid.New().String(), Email: "qsa-" + uuid.New().String() + "@test.com", Name: "QSA"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "squat", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded squat/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)

	for i, d := range []string{"2025-03-20", "2025-03-21"} {
		parsed, _ := time.Parse("2006-01-02", d)
		sess := &session.Session{UserID: u.ID, Date: parsed}
		if err := sessionRepo.Create(ctx, sess); err != nil {
			t.Fatal(err)
		}
		w := 135.0
		entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat"}
		sets := []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}, {Weight: &w, Reps: 5, SetOrder: 2}}
		if i == 1 {
			sets = append(sets, logentry.SetInput{Weight: &w, Reps: 5, SetOrder: 3})
		}
		if err := logentryRepo.Create(ctx, entry, sets); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "squat", Variant: "standard", Scope: "aggregate", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Entries != nil && len(res.Entries) > 0 {
		t.Errorf("aggregate should clear Entries, got %d", len(res.Entries))
	}
	if res.CountSessions != 2 {
		t.Errorf("got CountSessions %d, want 2", res.CountSessions)
	}
	if res.CountSets != 5 {
		t.Errorf("got CountSets %d, want 5", res.CountSets)
	}
	expectedVol := 135.0 * 5 * 5
	if res.TotalVolume != expectedVol {
		t.Errorf("got TotalVolume %v, want %v", res.TotalVolume, expectedVol)
	}
}

func TestService_Query_metricMaxWeight(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-mw-" + uuid.New().String(), Email: "qmw-" + uuid.New().String() + "@test.com", Name: "QMW"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "deadlift", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded deadlift/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w1, w2 := 185.0, 225.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "dl"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{
		{Weight: &w1, Reps: 5, SetOrder: 1},
		{Weight: &w2, Reps: 3, SetOrder: 2},
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "deadlift", Variant: "standard", Metric: "max_weight", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Metric != "max_weight" || res.Value == nil || *res.Value != 225 {
		t.Errorf("expected max_weight 225, got metric=%q value=%v", res.Metric, res.Value)
	}
}

func TestService_Query_metricLatestWeight(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-lw-" + uuid.New().String(), Email: "qlw-" + uuid.New().String() + "@test.com", Name: "QLW"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "deadlift", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded deadlift/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w1, w2 := 135.0, 185.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "dl"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{
		{Weight: &w1, Reps: 5, SetOrder: 1},
		{Weight: &w2, Reps: 5, SetOrder: 2},
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "deadlift", Variant: "standard", Metric: "latest_weight", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Metric != "latest_weight" || res.Value == nil || *res.Value != 185 {
		t.Errorf("expected latest_weight 185 (last set), got metric=%q value=%v", res.Metric, res.Value)
	}
}

func TestService_Query_metricEstimated1RM(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-1rm-" + uuid.New().String(), Email: "q1rm-" + uuid.New().String() + "@test.com", Name: "Q1RM"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded bench press/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w := 225.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "bench"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "bench press", Variant: "standard", Metric: "estimated_1rm", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	expected := 225 * (1 + 5.0/30)
	if res == nil || res.Metric != "estimated_1rm" || res.Value == nil {
		t.Fatalf("expected estimated_1rm, got metric=%q value=%v", res.Metric, res.Value)
	}
	if *res.Value < expected-1 || *res.Value > expected+1 {
		t.Errorf("expected ~%v, got %v", expected, *res.Value)
	}
}

func TestService_Query_metricCountSetsAndTotalVolume(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()
	u := &user.User{GoogleID: "q-cs-" + uuid.New().String(), Email: "qcs-" + uuid.New().String() + "@test.com", Name: "QCS"}
	userRepo := user.NewRepo(db)
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "squat", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded squat/standard:", err)
	}
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{
		{Weight: &w, Reps: 5, SetOrder: 1},
		{Weight: &w, Reps: 5, SetOrder: 2},
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(exerciseRepo, logentryRepo, sessionRepo)
	res, err := svc.Query(ctx, u.ID, QueryParams{Category: "squat", Variant: "standard", Metric: "count_sets", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Metric != "count_sets" || res.Value == nil || *res.Value != 2 {
		t.Errorf("expected count_sets 2, got metric=%q value=%v", res.Metric, res.Value)
	}

	res2, err := svc.Query(ctx, u.ID, QueryParams{Category: "squat", Variant: "standard", Metric: "total_volume", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	expectedVol := 135.0 * 5 * 2
	if res2 == nil || res2.Metric != "total_volume" || res2.Value == nil || *res2.Value != expectedVol {
		t.Errorf("expected total_volume %v, got %v", expectedVol, res2.Value)
	}
}
