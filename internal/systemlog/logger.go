package systemlog

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/google/uuid"
)

type contextKey int

const loggerContextKey contextKey = 0

// ContextWithLogger adds the logger to the request context.
func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// LoggerFromContext returns the logger from context, or nil.
func LoggerFromContext(ctx context.Context) Logger {
	l, _ := ctx.Value(loggerContextKey).(Logger)
	return l
}

// AddLoggerToContext returns middleware that adds the logger to the request context.
func AddLoggerToContext(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := ContextWithLogger(r.Context(), logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func debugStack() []byte { return debug.Stack() }

// RecoverPanic returns middleware that recovers from panics and logs them.
func RecoverPanic(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						stack := string(debug.Stack())
						logger.Log(r.Context(), InsertParams{
							Category: CategoryException,
							Method:   r.Method,
							Path:     r.URL.Path,
							Details:  map[string]interface{}{"panic": rec},
							Error:    stack,
						})
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal error", "code": "panic"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logger persists system log entries. May be nil for no-op.
type Logger interface {
	Log(ctx context.Context, p InsertParams)
}

type noopLogger struct{}

func (noopLogger) Log(ctx context.Context, p InsertParams) {}

// NoopLogger returns a logger that does nothing.
func NoopLogger() Logger { return noopLogger{} }

// RepoLogger wraps a Repo to implement Logger.
type RepoLogger struct {
	repo *Repo
}

func NewRepoLogger(repo *Repo) *RepoLogger {
	return &RepoLogger{repo: repo}
}

func (l *RepoLogger) Log(ctx context.Context, p InsertParams) {
	if l == nil || l.repo == nil {
		return
	}
	_ = l.repo.Insert(ctx, p)
}

// UserIDFromContext extracts user ID from context. Use auth.UserFromContext and pass u.ID.
type UserIDFromContext func(ctx context.Context) *uuid.UUID

// LogAction returns middleware that logs user actions after the handler runs.
// userFromCtx may be nil; if provided, it extracts the user ID for logging.
func LogAction(logger Logger, userFromCtx UserIDFromContext) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			if logger != nil {
				var userID *uuid.UUID
				if userFromCtx != nil {
					userID = userFromCtx(r.Context())
				}
				logger.Log(r.Context(), InsertParams{
					Category: CategoryAction,
					UserID:   userID,
					Method:   r.Method,
					Path:     r.URL.Path,
				})
			}
		})
	}
}
