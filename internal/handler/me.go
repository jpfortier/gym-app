package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jpfortier/gym-app/internal/auth"
)

func Me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":        u.ID.String(),
		"email":     u.Email,
		"name":      u.Name,
		"photo_url": u.PhotoURL,
	})
}
