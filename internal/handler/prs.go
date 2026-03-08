package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/pr"
)

func PRsList(prRepo *pr.Repo, exerciseRepo *exercise.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		prs, err := prRepo.ListByUser(r.Context(), u.ID)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}

		out := make([]map[string]any, len(prs))
		for i, p := range prs {
			categoryName := ""
			variantName := ""
			if v, _ := exerciseRepo.GetVariantByID(r.Context(), p.ExerciseVariantID); v != nil {
				variantName = v.Name
				if cat, _ := exerciseRepo.GetCategoryByID(r.Context(), v.CategoryID); cat != nil {
					categoryName = cat.Name
				}
			}
			entry := map[string]any{
				"id":                 p.ID.String(),
				"exercise_variant_id": p.ExerciseVariantID.String(),
				"exercise_name":      categoryName,
				"variant_name":       variantName,
				"pr_type":            p.PRType,
				"weight":             p.Weight,
				"reps":               p.Reps,
				"image_url":          p.ImageURL,
				"created_at":         p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
			if p.ImageURL == "" {
				entry["image_url"] = nil
			}
			out[i] = entry
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}
