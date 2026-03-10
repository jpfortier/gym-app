package auth

import (
	"net/http"

	"github.com/jpfortier/gym-app/internal/httputil"
)

// RequireAdmin returns middleware that wraps RequireAuth and additionally
// checks user.Role == "admin". Returns 403 if not admin.
func RequireAdmin(verifier Verifier, userStore UserStore, googleClientID string) func(http.Handler) http.Handler {
	auth := RequireAuth(verifier, userStore, googleClientID)
	return func(next http.Handler) http.Handler {
		return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFromContext(r.Context())
			if u == nil {
				httputil.JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
				return
			}
			if u.Role != "admin" {
				httputil.JSONError(w, "admin access required", "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}
