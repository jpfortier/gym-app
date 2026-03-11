package exercise

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/testutil"
	"github.com/jpfortier/gym-app/internal/user"
)

func dbForTest(t *testing.T) *sql.DB { return testutil.DBForTest(t) }

func TestRepo_Resolve_seededGlobal(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "ex")

	repo := NewRepo(db)
	v, err := repo.Resolve(ctx, u.ID, "bench press", "standard")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		t.Fatal("expected variant from seeded bench press / standard")
	}
	if v.Name != "bench press" {
		t.Errorf("got variant name %q, want bench press (standard variant)", v.Name)
	}
}

func TestRepo_Resolve_caseInsensitive(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createExerciseTestUser(t, db, ctx)

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

func TestRepo_StoreAlias_FindVariantByAlias(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := createExerciseTestUser(t, db, ctx)

	repo := NewRepo(db)
	cat := &Category{UserID: &u.ID, Name: "Deadlift", ShowWeight: true, ShowReps: true}
	if err := repo.CreateCategory(ctx, cat); err != nil {
		t.Fatal(err)
	}
	variant := &Variant{CategoryID: cat.ID, UserID: &u.ID, Name: "rdl"}
	if err := repo.CreateVariant(ctx, variant); err != nil {
		t.Fatal(err)
	}

	if err := repo.StoreAlias(ctx, u.ID, "rdl standard", variant.ID); err != nil {
		t.Fatal(err)
	}

	v, err := repo.FindVariantByAlias(ctx, u.ID, "rdl standard")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.ID != variant.ID {
		t.Errorf("expected variant from alias, got %+v", v)
	}

	v, err = repo.FindVariantByAlias(ctx, u.ID, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("expected nil for nonexistent alias, got %+v", v)
	}
}

func createExerciseTestUser(t *testing.T, db *sql.DB, ctx context.Context) *user.User {
	return testutil.CreateTestUser(t, db, ctx, "ex")
}
