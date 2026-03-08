package session

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/env"
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

func TestRepo_Create_GetByID(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "session-test-" + uuid.New().String(), Email: "session@test.com", Name: "Session Test"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	s := &Session{UserID: u.ID, Date: time.Date(2025, 3, 7, 0, 0, 0, 0, time.UTC)}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	if s.ID == uuid.Nil {
		t.Error("expected ID to be set after Create")
	}
	if s.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	got, err := repo.GetByID(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.ID != s.ID || got.UserID != u.ID || got.Date.Format("2006-01-02") != "2025-03-07" {
		t.Errorf("got %+v, want ID=%s UserID=%s Date=2025-03-07", got, s.ID, u.ID)
	}
}

func TestRepo_GetByID_notFound(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	repo := NewRepo(db)
	got, err := repo.GetByID(ctx, uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestRepo_GetByUserAndDate(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "session-by-date-" + uuid.New().String(), Email: "by-date@test.com", Name: "By Date"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	s := &Session{UserID: u.ID, Date: time.Date(2025, 3, 8, 0, 0, 0, 0, time.UTC)}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByUserAndDate(ctx, u.ID, "2025-03-08")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.ID != s.ID {
		t.Errorf("got ID %s, want %s", got.ID, s.ID)
	}
}

func TestRepo_GetByUserAndDate_notFound(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "session-nf-" + uuid.New().String(), Email: "nf@test.com", Name: "NF"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	got, err := repo.GetByUserAndDate(ctx, u.ID, "2025-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestRepo_ListByUser(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "session-list-" + uuid.New().String(), Email: "list@test.com", Name: "List"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	repo := NewRepo(db)
	for i, d := range []string{"2025-03-05", "2025-03-06", "2025-03-07"} {
		parsed, _ := time.Parse("2006-01-02", d)
		s := &Session{UserID: u.ID, Date: parsed}
		if err := repo.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
		_ = i
	}

	list, err := repo.ListByUser(ctx, u.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("got %d sessions, want 3", len(list))
	}
	if len(list) >= 2 && list[0].Date.Before(list[1].Date) {
		t.Error("expected list ordered by date DESC")
	}
}
