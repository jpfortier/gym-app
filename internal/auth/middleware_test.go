package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/user"
	"google.golang.org/api/idtoken"
)

func TestRequireAuth_missingAuthorization(t *testing.T) {
	verifier := &mockVerifier{}
	store := &mockUserStore{}
	handler := RequireAuth(verifier, store, "test-client-id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
	if rec.Body.String() != `{"error":"missing authorization"}`+"\n" {
		t.Errorf("got body %q", rec.Body.String())
	}
	if verifier.verifyCalled {
		t.Error("verifier should not be called when token is missing")
	}
}

func TestRequireAuth_malformedBearer(t *testing.T) {
	verifier := &mockVerifier{}
	store := &mockUserStore{}
	handler := RequireAuth(verifier, store, "test-client-id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Basic xyz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
	if verifier.verifyCalled {
		t.Error("verifier should not be called for non-Bearer auth")
	}
}

func TestRequireAuth_invalidToken(t *testing.T) {
	verifier := &mockVerifier{err: errInvalidToken}
	store := &mockUserStore{}
	handler := RequireAuth(verifier, store, "test-client-id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", rec.Code)
	}
	if rec.Body.String() != `{"error":"invalid token"}`+"\n" {
		t.Errorf("got body %q", rec.Body.String())
	}
}

func TestRequireAuth_validToken_existingUser(t *testing.T) {
	existingUser := &user.User{
		ID:       uuid.MustParse("a1b2c3d4-e5f6-4a5b-8c9d-0e1f2a3b4c5d"),
		GoogleID: "google-123",
		Email:    "test@example.com",
		Name:     "Test User",
		PhotoURL: "https://example.com/photo.jpg",
		CreatedAt: time.Now(),
	}

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: "google-123"}}
	store := &mockUserStore{user: existingUser}
	var capturedUser *user.User
	handler := RequireAuth(verifier, store, "test-client-id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("user should be in context")
	}
	if capturedUser.ID != existingUser.ID {
		t.Errorf("got user id %s, want %s", capturedUser.ID, existingUser.ID)
	}
	if capturedUser.Email != "test@example.com" {
		t.Errorf("got email %q, want test@example.com", capturedUser.Email)
	}
	if !store.getByGoogleIDCalled {
		t.Error("GetByGoogleID should have been called")
	}
	if store.createCalled {
		t.Error("Create should not be called for existing user")
	}
}

func TestRequireAuth_validToken_createsNewUser(t *testing.T) {
	verifier := &mockVerifier{
		payload: &idtoken.Payload{
			Subject: "google-new",
			Claims: map[string]interface{}{
				"email":   "new@example.com",
				"name":   "New User",
				"picture": "https://example.com/new.jpg",
			},
		},
	}
	store := &mockUserStore{}
	var capturedUser *user.User
	handler := RequireAuth(verifier, store, "test-client-id")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("user should be in context")
	}
	if capturedUser.GoogleID != "google-new" {
		t.Errorf("got google_id %q, want google-new", capturedUser.GoogleID)
	}
	if capturedUser.Email != "new@example.com" {
		t.Errorf("got email %q, want new@example.com", capturedUser.Email)
	}
	if capturedUser.Name != "New User" {
		t.Errorf("got name %q, want New User", capturedUser.Name)
	}
	if capturedUser.PhotoURL != "https://example.com/new.jpg" {
		t.Errorf("got photo_url %q", capturedUser.PhotoURL)
	}
	if !store.createCalled {
		t.Error("Create should have been called for new user")
	}
}

var errInvalidToken = context.DeadlineExceeded

type mockVerifier struct {
	payload       *idtoken.Payload
	err           error
	verifyCalled  bool
}

func (m *mockVerifier) Verify(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
	m.verifyCalled = true
	if m.err != nil {
		return nil, m.err
	}
	return m.payload, nil
}

type mockUserStore struct {
	user               *user.User
	getByGoogleIDCalled bool
	createCalled        bool
}

func (m *mockUserStore) GetByGoogleID(ctx context.Context, googleID string) (*user.User, error) {
	m.getByGoogleIDCalled = true
	return m.user, nil
}

func (m *mockUserStore) Create(ctx context.Context, u *user.User) error {
	m.createCalled = true
	if u.ID == uuid.Nil {
		u.ID = uuid.Must(uuid.NewV7())
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	return nil
}
