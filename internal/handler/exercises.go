package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
)

func ExercisesList(exerciseRepo *exercise.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, r, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}

		categories, err := exerciseRepo.ListCategoriesForUser(r.Context(), u.ID)
		if err != nil {
			JSONError(w, r, "internal error", "internal_error", http.StatusInternalServerError)
			return
		}

		out := make([]map[string]any, 0, len(categories)*2)
		for _, c := range categories {
			variants, err := exerciseRepo.ListVariantsByCategory(r.Context(), c.ID, u.ID)
			if err != nil {
				JSONError(w, r, "internal error", "internal_error", http.StatusInternalServerError)
				return
			}
			for _, v := range variants {
				m := map[string]any{
					"category_id":   c.ID.String(),
					"category_name": c.Name,
					"variant_id":    v.ID.String(),
					"variant_name":  v.Name,
					"show_weight":   c.ShowWeight,
					"show_reps":     c.ShowReps,
				}
				if v.VisualCues != "" {
					m["visual_cues"] = v.VisualCues
				}
				out = append(out, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}
