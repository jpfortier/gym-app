package handler

import (
	"net/http"

	"github.com/jpfortier/gym-app/internal/httputil"
)

// JSONError writes a JSON error response with error_token for debugging.
func JSONError(w http.ResponseWriter, r *http.Request, msg, code string, status int) {
	httputil.JSONError(w, r, msg, code, status)
}
