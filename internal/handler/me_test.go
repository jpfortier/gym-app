package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestMe_noUserInContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
	if rec.Body.String() != `{"error":"unauthorized"}`+"\n" {
		t.Errorf("got body %q", rec.Body.String())
	}
}

func TestMe_userInContext(t *testing.T) {
	u := &user.User{
		ID:        uuid.MustParse("a1b2c3d4-e5f6-4a5b-8c9d-0e1f2a3b4c5d"),
		GoogleID:  "google-123",
		Email:     "test@example.com",
		Name:      "Test User",
		PhotoURL:  "https://example.com/photo.jpg",
		CreatedAt: time.Now(),
	}
	ctx := auth.ContextWithUser(context.Background(), u)
	req := httptest.NewRequest(http.MethodGet, "/me", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", ct)
	}
	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected JSON body")
	}
	if !strings.Contains(body, "test@example.com") {
		t.Errorf("body %q should contain email", body)
	}
	if !strings.Contains(body, u.ID.String()) {
		t.Errorf("body %q should contain user id", body)
	}
}
