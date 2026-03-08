package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// JSONError writes a JSON error response with error_token for debugging.
func JSONError(w http.ResponseWriter, msg, code string, status int) {
	token := "err_" + uuid.Must(uuid.NewV7()).String()[:12]
	slog.Error("handler error", "error_token", token, "code", code, "msg", msg)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":       msg,
		"code":        code,
		"error_token": token,
	})
}
