package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/systemlog"
)

// JSONError writes a JSON error response with error_token for debugging.
// If r is non-nil and a system logger is in context, also persists to system_logs.
func JSONError(w http.ResponseWriter, r *http.Request, msg, code string, status int) {
	token := "err_" + uuid.Must(uuid.NewV7()).String()[:12]
	slog.Error("handler error", "error_token", token, "code", code, "msg", msg)
	if r != nil {
		if logger := systemlog.LoggerFromContext(r.Context()); logger != nil {
			logger.Log(r.Context(), systemlog.InsertParams{
				Category: systemlog.CategoryException,
				Method:   r.Method,
				Path:     r.URL.Path,
				Details:  map[string]interface{}{"code": code, "error_token": token},
				Error:    msg,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":       msg,
		"code":        code,
		"error_token": token,
	})
}
