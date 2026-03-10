package admin

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	selectedUserCookie = "gym_admin_selected_user"
	authTokenCookie    = "gym_admin_token"
)

// SelectedUserMaxAge is the cookie max age in seconds (30 days).
const SelectedUserMaxAge = 30 * 24 * 60 * 60

// ReadSelectedUser returns the selected user ID from the cookie, or nil if not set.
func ReadSelectedUser(r *http.Request) *uuid.UUID {
	c, err := r.Cookie(selectedUserCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	id, err := uuid.Parse(c.Value)
	if err != nil {
		return nil
	}
	return &id
}

// SetSelectedUser sets the selected user cookie and redirects.
// If userID is empty, clears the cookie.
func SetSelectedUserCookie(w http.ResponseWriter, userID string) {
	c := &http.Cookie{
		Name:     selectedUserCookie,
		Path:     "/admin",
		MaxAge:   SelectedUserMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if userID != "" {
		c.Value = userID
	} else {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

// InjectAuthFromCookie adds the auth token from cookie to the request header if not present.
func InjectAuthFromCookie(r *http.Request) {
	if r.Header.Get("Authorization") != "" {
		return
	}
	c, err := r.Cookie(authTokenCookie)
	if err != nil || c.Value == "" {
		return
	}
	r.Header.Set("Authorization", "Bearer "+c.Value)
}

// InjectAuthCookie returns middleware that injects auth token from cookie before calling next.
// Use when wrapping admin routes so browser requests with cookie-based auth work.
func InjectAuthCookie(nextMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			InjectAuthFromCookie(r)
			nextMiddleware(next).ServeHTTP(w, r)
		})
	}
}

// SetAuthCookie sets the auth token cookie.
func SetAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     authTokenCookie,
		Value:    token,
		Path:     "/admin",
		MaxAge:   int(24 * time.Hour.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearAuthCookie clears the auth token cookie.
func ClearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authTokenCookie,
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
