package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
)

func SessionsList(sessionRepo *session.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}

		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}

		sessions, err := sessionRepo.ListByUser(r.Context(), u.ID, limit)
		if err != nil {
			JSONError(w, "internal error", "internal_error", http.StatusInternalServerError)
			return
		}

		out := make([]map[string]any, len(sessions))
		for i, s := range sessions {
			out[i] = map[string]any{
				"id":         s.ID.String(),
				"date":       s.Date.Format("2006-01-02"),
				"created_at": s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func SessionDetail(sessionRepo *session.Repo, logentryRepo *logentry.Repo, exerciseRepo *exercise.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := r.PathValue("id")
		if idStr == "" {
			JSONError(w, "missing session id", "invalid_input", http.StatusBadRequest)
			return
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			JSONError(w, "invalid session id", "invalid_input", http.StatusBadRequest)
			return
		}

		sess, err := sessionRepo.GetByID(r.Context(), id)
		if err != nil {
			JSONError(w, "internal error", "internal_error", http.StatusInternalServerError)
			return
		}
		if sess == nil || sess.UserID != u.ID {
			JSONError(w, "not found", "not_found", http.StatusNotFound)
			return
		}

		entries, err := logentryRepo.ListBySession(r.Context(), sess.ID)
		if err != nil {
			JSONError(w, "internal error", "internal_error", http.StatusInternalServerError)
			return
		}

		entryOut := make([]map[string]any, len(entries))
		for i, e := range entries {
			variant, _ := exerciseRepo.GetVariantByID(r.Context(), e.ExerciseVariantID)
			categoryName := ""
			variantName := e.ExerciseVariantID.String()
			if variant != nil {
				variantName = variant.Name
				if cat, _ := exerciseRepo.GetCategoryByID(r.Context(), variant.CategoryID); cat != nil {
					categoryName = cat.Name
				}
			}

			setsOut := make([]map[string]any, len(e.Sets))
			for j, set := range e.Sets {
				setsOut[j] = map[string]any{
					"weight":    set.Weight,
					"reps":      set.Reps,
					"set_type":  set.SetType,
				}
			}

			entryOut[i] = map[string]any{
				"id":                 e.ID.String(),
				"exercise_variant_id": e.ExerciseVariantID.String(),
				"exercise_name":      categoryName,
				"variant_name":       variantName,
				"raw_speech":         e.RawSpeech,
				"notes":              e.Notes,
				"sets":               setsOut,
			}
		}

		out := map[string]any{
			"id":         sess.ID.String(),
			"date":       sess.Date.Format("2006-01-02"),
			"created_at": sess.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"entries":    entryOut,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}
