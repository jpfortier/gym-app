package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jpfortier/gym-app/internal/user"
	"google.golang.org/api/idtoken"
)

type contextKey int

const userContextKey contextKey = 0

// UserStore gets or creates users. Use *user.Repo in production.
type UserStore interface {
	GetByGoogleID(ctx context.Context, googleID string) (*user.User, error)
	Create(ctx context.Context, u *user.User) error
}

// UserFromContext returns the authenticated user from the request context.
// Returns nil if no user is set (e.g. unauthenticated request).
func UserFromContext(ctx context.Context) *user.User {
	u, _ := ctx.Value(userContextKey).(*user.User)
	return u
}

// ContextWithUser returns a context with the user set. For testing only.
func ContextWithUser(ctx context.Context, u *user.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// RequireAuth returns middleware that verifies the Google ID token, gets or creates
// the user, and adds the user to the request context. Responds 401 if auth fails.
func RequireAuth(verifier Verifier, userStore UserStore, googleClientID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				slog.Debug("auth: missing bearer token")
				http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
				return
			}

			payload, err := verifier.Verify(r.Context(), token, googleClientID)
			if err != nil {
				slog.Debug("auth: invalid token", "err", err)
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			u, err := getOrCreateUser(r.Context(), userStore, payload)
			if err != nil {
				slog.Error("auth: get or create user", "err", err)
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

func getOrCreateUser(ctx context.Context, store UserStore, payload *idtoken.Payload) (*user.User, error) {
	u, err := store.GetByGoogleID(ctx, payload.Subject)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	u = &user.User{
		GoogleID: payload.Subject,
		Email:   email,
		Name:    name,
		PhotoURL: picture,
	}
	if err := store.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
