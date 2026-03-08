package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/query"
)

func QueryHistory(queryService *query.Service, exerciseRepo *exercise.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}

		category := r.URL.Query().Get("category")
		if category == "" {
			category = r.URL.Query().Get("exercise")
		}
		variant := r.URL.Query().Get("variant")
		if variant == "" {
			variant = "standard"
		}
		if category == "" {
			JSONError(w, "category or exercise required", "invalid_input", http.StatusBadRequest)
			return
		}

		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 50 {
				limit = n
			}
		}
		fromDate := r.URL.Query().Get("from")
		toDate := r.URL.Query().Get("to")

		entries, variantOut, err := queryService.History(r.Context(), u.ID, category, variant, fromDate, toDate, limit)
		if err != nil {
			JSONError(w, "internal error", "internal_error", http.StatusInternalServerError)
			return
		}
		if variantOut == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"entries": []any{}, "exercise_name": "", "variant_name": ""})
			return
		}

		categoryName := ""
		if cat, _ := exerciseRepo.GetCategoryByID(r.Context(), variantOut.CategoryID); cat != nil {
			categoryName = cat.Name
		}

		entryOut := make([]map[string]any, len(entries))
		for i, e := range entries {
			setsOut := make([]map[string]any, len(e.Sets))
			for j, s := range e.Sets {
				setsOut[j] = map[string]any{"weight": s.Weight, "reps": s.Reps, "set_type": s.SetType}
			}
			entryOut[i] = map[string]any{
				"session_date": e.SessionDate,
				"raw_speech":   e.RawSpeech,
				"sets":         setsOut,
				"created_at":   e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}

		out := map[string]any{
			"entries":       entryOut,
			"exercise_name": categoryName,
			"variant_name":  variantOut.Name,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}
