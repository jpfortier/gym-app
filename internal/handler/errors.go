package handler

import (
	"net/http"

	"github.com/jpfortier/gym-app/internal/httputil"
)

// JSONError writes a JSON error response with error_token for debugging.
func JSONError(w http.ResponseWriter, msg, code string, status int) {
	httputil.JSONError(w, msg, code, status)
}
