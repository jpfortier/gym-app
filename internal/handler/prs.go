package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/storage"
)

func PRsList(prRepo *pr.Repo, exerciseRepo *exercise.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}

		prs, err := prRepo.ListByUser(r.Context(), u.ID)
		if err != nil {
			JSONError(w, "internal error", "internal_error", http.StatusInternalServerError)
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

func PRImage(prRepo *pr.Repo, r2 *storage.R2) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}
		idStr := r.PathValue("id")
		if idStr == "" {
			JSONError(w, "id required", "invalid_input", http.StatusBadRequest)
			return
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			JSONError(w, "invalid id", "invalid_input", http.StatusBadRequest)
			return
		}
		prRec, err := prRepo.GetByID(r.Context(), id)
		if err != nil || prRec == nil {
			JSONError(w, "not found", "not_found", http.StatusNotFound)
			return
		}
		if prRec.UserID != u.ID {
			JSONError(w, "not found", "not_found", http.StatusNotFound)
			return
		}
		if prRec.ImageURL == "" {
			JSONError(w, "image not ready", "not_found", http.StatusNotFound)
			return
		}
		if r2 == nil {
			JSONError(w, "storage not configured", "internal_error", http.StatusServiceUnavailable)
			return
		}
		url, err := r2.PresignPRImage(r.Context(), u.ID, id, 3600)
		if err != nil {
			JSONError(w, "failed to generate URL", "internal_error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
	}
}

