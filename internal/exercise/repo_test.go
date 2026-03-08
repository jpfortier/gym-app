package exercise

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

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

func TestRepo_Resolve_seededGlobal(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "ex-" + uuid.New().String(), Email: "ex@test.com", Name: "Ex"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	repo := NewRepo(db)
	v, err := repo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		t.Fatal("expected variant from seeded bench press / standard")
	}
	if v.Name != "standard" {
		t.Errorf("got variant name %q, want standard", v.Name)
	}
}

func TestRepo_Resolve_caseInsensitive(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createExerciseTestUser(t, db, ctx)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	v, err := repo.Resolve(ctx, u.ID, "DEADLIFT", "STANDARD")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		t.Fatal("expected variant (case insensitive)")
	}
}

func TestRepo_Resolve_notFound(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createExerciseTestUser(t, db, ctx)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	v, err := repo.Resolve(ctx, u.ID, "nonexistent exercise", "standard")
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("expected nil for nonexistent category, got %+v", v)
	}
}

func TestRepo_CreateCategory_CreateVariant_Resolve(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createExerciseTestUser(t, db, ctx)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	cat := &Category{UserID: &u.ID, Name: "hula hoop", ShowWeight: true, ShowReps: true}
	if err := repo.CreateCategory(ctx, cat); err != nil {
		t.Fatal(err)
	}
	variant := &Variant{CategoryID: cat.ID, UserID: &u.ID, Name: "overhead"}
	if err := repo.CreateVariant(ctx, variant); err != nil {
		t.Fatal(err)
	}

	v, err := repo.Resolve(ctx, u.ID, "hula hoop", "overhead")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.ID != variant.ID {
		t.Errorf("expected resolved user variant: got %+v", v)
	}
}

func createExerciseTestUser(t *testing.T, db *sql.DB, ctx context.Context) *user.User {
	t.Helper()
	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "ex-" + uuid.New().String(), Email: "ex@test.com", Name: "Ex"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	return u
}
