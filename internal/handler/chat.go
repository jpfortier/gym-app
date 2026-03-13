package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
)

func Chat(chatSvc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			JSONError(w, r, "method not allowed", "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, r, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Text        string `json:"text"`
			AudioBase64 string `json:"audio_base64"`
			AudioFormat string `json:"audio_format"` // e.g. "m4a", "webm" - optional
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			JSONError(w, r, "invalid JSON", "invalid_input", http.StatusBadRequest)
			return
		}
		resp, jobResp, err := chatSvc.Process(r.Context(), u, req.Text, req.AudioBase64, req.AudioFormat)
		if err != nil {
			slog.Error("chat process failed", "err", err)
			JSONError(w, r, "Processing failed", "internal_error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if jobResp != nil {
			_ = json.NewEncoder(w).Encode(jobResp)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func ChatJob(chatSvc *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			JSONError(w, r, "method not allowed", "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			JSONError(w, r, "unauthorized", "unauthorized", http.StatusUnauthorized)
			return
		}
		idStr := r.PathValue("id")
		if idStr == "" {
			JSONError(w, r, "missing job id", "invalid_input", http.StatusBadRequest)
			return
		}
		jobID, err := uuid.Parse(idStr)
		if err != nil {
			JSONError(w, r, "invalid job id", "invalid_input", http.StatusBadRequest)
			return
		}
		job, ok := chatSvc.GetJob(jobID, u.ID)
		if !ok {
			JSONError(w, r, "job not found", "not_found", http.StatusNotFound)
			return
		}
		jobResp := chat.JobResponse{
			JobID:  job.ID.String(),
			Text:   job.Text,
			Status: job.Status,
			Result: job.Result,
			Error:  job.Error,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jobResp)
	}
}
