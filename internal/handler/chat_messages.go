package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chatmessages"
)

func ChatMessages(chatMessagesRepo *chatmessages.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			JSONError(w, "method not allowed", "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}

		limit := 6
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 50 {
				limit = n
			}
		}

		beforeStr := r.URL.Query().Get("before")
		var msgs []chatmessages.Message
		var err error
		if beforeStr == "" {
			msgs, err = chatMessagesRepo.ListRecent(r.Context(), u.ID, limit)
		} else {
			beforeID, parseErr := uuid.Parse(beforeStr)
			if parseErr != nil {
				JSONError(w, "invalid before id", "invalid_input", http.StatusBadRequest)
				return
			}
			msgs, err = chatMessagesRepo.ListOlder(r.Context(), u.ID, beforeID, limit)
		}
		if err != nil {
			JSONError(w, "internal error", "internal_error", http.StatusInternalServerError)
			return
		}

		out := make([]map[string]any, len(msgs))
		for i, m := range msgs {
			out[i] = map[string]any{
				"id":         m.ID.String(),
				"role":       m.Role,
				"content":    m.Content,
				"created_at": m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": out})
	}
}
