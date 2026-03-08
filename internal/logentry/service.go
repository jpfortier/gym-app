package logentry

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/session"
)

type Service struct {
	repo           *Repo
	sessionService *session.Service
}

func NewService(repo *Repo, sessionService *session.Service) *Service {
	return &Service{repo: repo, sessionService: sessionService}
}

// CreateLogEntry creates a session for the date if needed, then creates the log entry with sets.
func (s *Service) CreateLogEntry(ctx context.Context, userID uuid.UUID, date string, variantID uuid.UUID, rawSpeech, notes string, sets []SetInput) (*LogEntry, error) {
	sess, err := s.sessionService.GetOrCreateForDate(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("get or create session: %w", err)
	}
	entry := &LogEntry{
		SessionID:         sess.ID,
		ExerciseVariantID: variantID,
		RawSpeech:         rawSpeech,
		Notes:             notes,
	}
	if err := s.repo.Create(ctx, entry, sets); err != nil {
		return nil, fmt.Errorf("create log entry: %w", err)
	}
	return entry, nil
}
