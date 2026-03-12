package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/httputil"
	"github.com/jpfortier/gym-app/internal/systemlog"
	"github.com/jpfortier/gym-app/internal/user"
	"google.golang.org/api/idtoken"
)

type contextKey int

const userContextKey contextKey = 0

// UserStore gets or creates users. Use *user.Repo in production.
type UserStore interface {
	GetByGoogleID(ctx context.Context, googleID string) (*user.User, error)
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	Create(ctx context.Context, u *user.User) error
	UpdateGoogleID(ctx context.Context, userID uuid.UUID, googleID string) error
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
// logger may be nil to skip system logging.
func RequireAuth(verifier Verifier, userStore UserStore, googleClientID string, logger systemlog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, path := r.Method, r.URL.Path
			token := extractBearerToken(r)
			if token == "" {
				slog.Debug("auth: missing bearer token")
				if logger != nil {
					logger.Log(r.Context(), systemlog.InsertParams{
						Category: systemlog.CategoryAuthFailure,
						Method:   method,
						Path:     path,
						Details:  map[string]interface{}{"reason": "missing_auth"},
					})
				}
				httputil.JSONError(w, r, "missing authorization", "missing_auth", http.StatusUnauthorized)
				return
			}

			if env.DevMode() && strings.HasPrefix(token, "dev:") {
				email := strings.TrimSpace(strings.TrimPrefix(token, "dev:"))
				if email == "" {
					if logger != nil {
						logger.Log(r.Context(), systemlog.InsertParams{
							Category: systemlog.CategoryAuthFailure,
							Method:   method,
							Path:     path,
							Details:  map[string]interface{}{"reason": "invalid_token", "msg": "dev token requires email"},
						})
					}
					httputil.JSONError(w, r, "dev token requires email", "invalid_token", http.StatusUnauthorized)
					return
				}
				u, err := getOrCreateDevUser(r.Context(), userStore, email)
				if err != nil {
					slog.Error("auth: dev user", "err", err)
					if logger != nil {
						logger.Log(r.Context(), systemlog.InsertParams{
							Category: systemlog.CategoryAuthFailure,
							Method:   method,
							Path:     path,
							Error:   err.Error(),
						})
					}
					httputil.JSONError(w, r, "internal error", "internal_error", http.StatusInternalServerError)
					return
				}
				if logger != nil {
					logger.Log(r.Context(), systemlog.InsertParams{
						Category: systemlog.CategoryAuthSuccess,
						UserID:   &u.ID,
						Method:   method,
						Path:     path,
						Details:  map[string]interface{}{"email": u.Email, "method": "dev"},
					})
				}
				ctx := context.WithValue(r.Context(), userContextKey, u)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			payload, err := verifier.Verify(r.Context(), token, googleClientID)
			if err != nil {
				slog.Debug("auth: invalid token", "err", err)
				if logger != nil {
					logger.Log(r.Context(), systemlog.InsertParams{
						Category: systemlog.CategoryAuthFailure,
						Method:   method,
						Path:     path,
						Details:  map[string]interface{}{"reason": "invalid_token"},
						Error:   err.Error(),
					})
				}
				httputil.JSONError(w, r, "invalid token", "invalid_token", http.StatusUnauthorized)
				return
			}

			u, err := getOrCreateUser(r.Context(), userStore, payload)
			if err != nil {
				slog.Error("auth: get or create user", "err", err)
				if logger != nil {
					logger.Log(r.Context(), systemlog.InsertParams{
						Category: systemlog.CategoryAuthFailure,
						Method:   method,
						Path:     path,
						Error:   err.Error(),
					})
				}
				httputil.JSONError(w, r, "internal error", "internal_error", http.StatusInternalServerError)
				return
			}

			if logger != nil {
				logger.Log(r.Context(), systemlog.InsertParams{
					Category: systemlog.CategoryAuthSuccess,
					UserID:   &u.ID,
					Method:   method,
					Path:     path,
					Details:  map[string]interface{}{"email": u.Email, "method": "google"},
				})
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

	u, err = store.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u != nil {
		if err := store.UpdateGoogleID(ctx, u.ID, payload.Subject); err != nil {
			return nil, err
		}
		u.GoogleID = payload.Subject
		if name != "" {
			u.Name = name
		}
		if picture != "" {
			u.PhotoURL = picture
		}
		return u, nil
	}

	u = &user.User{
		GoogleID: payload.Subject,
		Email:    email,
		Name:     name,
		PhotoURL: picture,
	}
	if err := store.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func getOrCreateDevUser(ctx context.Context, store UserStore, email string) (*user.User, error) {
	u, err := store.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}
	u = &user.User{
		GoogleID: "dev-" + email,
		Email:    email,
		Name:     "",
	}
	if err := store.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
