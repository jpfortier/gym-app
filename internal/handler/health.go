package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/jpfortier/gym-app/internal/env"
)

func Health(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": "database"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": env.BuildDate()})
	}
}
