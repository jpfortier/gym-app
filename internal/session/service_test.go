package session

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/testutil"
)

func TestService_GetOrCreateForDate_createsNew(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "svc-test")

	svc := NewService(NewRepo(db))
	sess, err := svc.GetOrCreateForDate(ctx, u.ID, "2025-03-09")
	if err != nil {
		t.Fatal(err)
	}
	if sess == nil {
		t.Fatal("expected session")
	}
	if sess.ID == uuid.Nil {
		t.Error("expected ID set")
	}
	if sess.UserID != u.ID {
		t.Errorf("UserID %s != %s", sess.UserID, u.ID)
	}
	if sess.Date.Format("2006-01-02") != "2025-03-09" {
		t.Errorf("Date %s != 2025-03-09", sess.Date.Format("2006-01-02"))
	}
}

func TestService_GetOrCreateForDate_returnsExisting(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "svc-test")

	repo := NewRepo(db)
	svc := NewService(repo)

	s1, err := svc.GetOrCreateForDate(ctx, u.ID, "2025-03-10")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := svc.GetOrCreateForDate(ctx, u.ID, "2025-03-10")
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID != s2.ID {
		t.Errorf("expected same session, got %s and %s", s1.ID, s2.ID)
	}
}

func TestService_GetOrCreateForDate_repoAndServiceIntegration(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "svc-test")

	repo := NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-11")
	sess := &Session{UserID: u.ID, Date: parsed}
	if err := repo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	svc := NewService(repo)
	got, err := svc.GetOrCreateForDate(ctx, u.ID, "2025-03-11")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sess.ID {
		t.Errorf("service should return existing session from repo: got ID %s, want %s", got.ID, sess.ID)
	}
}

func TestService_GetOrCreateForDate_invalidDate(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "svc-test")

	svc := NewService(NewRepo(db))
	_, err := svc.GetOrCreateForDate(ctx, u.ID, "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

