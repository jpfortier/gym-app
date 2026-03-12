package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/user"
	"google.golang.org/api/idtoken"
)

func TestRequireAdmin_nonAdminReturns403(t *testing.T) {
	u := &user.User{
		ID:        uuid.Must(uuid.NewV7()),
		GoogleID:  "google-123",
		Email:     "user@example.com",
		Role:      "user",
		CreatedAt: time.Now(),
	}
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: "google-123"}}
	store := &mockUserStore{user: u}

	handler := RequireAdmin(verifier, store, "test-client-id", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for non-admin")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("got status %d, want 403", rec.Code)
	}
	var errBody struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if errBody.Code != "forbidden" {
		t.Errorf("got code %q, want forbidden", errBody.Code)
	}
}

func TestRequireAdmin_adminPassesThrough(t *testing.T) {
	u := &user.User{
		ID:        uuid.Must(uuid.NewV7()),
		GoogleID:  "google-admin",
		Email:     "admin@example.com",
		Role:      "admin",
		CreatedAt: time.Now(),
	}
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: "google-admin"}}
	store := &mockUserStore{user: u}

	var handlerCalled bool
	handler := RequireAdmin(verifier, store, "test-client-id", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		cu := UserFromContext(r.Context())
		if cu == nil || cu.Role != "admin" {
			t.Error("user should be admin in context")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !handlerCalled {
		t.Error("handler should have been called for admin")
	}
}
