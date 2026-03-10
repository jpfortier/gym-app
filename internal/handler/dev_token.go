package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jpfortier/gym-app/internal/env"
)

// DevToken returns a dev token for automated tests. Only when GYM_DEV_MODE=true.
// GET /dev/token?email=test@example.com
// Returns {"token": "dev:test@example.com"}. Use as Authorization: Bearer <token>
func DevToken(w http.ResponseWriter, r *http.Request) {
	if !env.DevMode() {
		JSONError(w, "not available", "not_found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		JSONError(w, "method not allowed", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		email = "test@example.com"
	}
	token := "dev:" + email
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"token":      token,
		"build_date": env.BuildDate(),
	})
}
