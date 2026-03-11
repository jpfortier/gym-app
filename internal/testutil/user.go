package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/user"
)

// CreateTestUser creates a user with @test.com email and registers t.Cleanup to delete it.
// Deletion order: workout_sessions first (cascades to log_entries), then users.
func CreateTestUser(t *testing.T, db *sql.DB, ctx context.Context, prefix string) *user.User {
	t.Helper()
	repo := user.NewRepo(db)
	u := &user.User{
		GoogleID: prefix + "-" + uuid.New().String(),
		Email:    prefix + "-" + uuid.New().String() + "@test.com",
		Name:     prefix,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { CleanupTestUser(t, db, ctx, u.ID) })
	return u
}

// CleanupTestUser deletes a test user and related data in the correct order.
// Call via t.Cleanup when creating users manually.
func CleanupTestUser(t *testing.T, db *sql.DB, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	_, _ = db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", userID)
	_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
}

// CleanupTestUsers deletes multiple test users. Call via t.Cleanup when creating users manually.
func CleanupTestUsers(t *testing.T, db *sql.DB, ctx context.Context, userIDs ...uuid.UUID) {
	t.Helper()
	for _, id := range userIDs {
		CleanupTestUser(t, db, ctx, id)
	}
}
