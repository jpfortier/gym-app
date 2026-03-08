package query

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
)

type Service struct {
	exerciseRepo *exercise.Repo
	logentryRepo *logentry.Repo
	sessionRepo  *session.Repo
}

func NewService(exerciseRepo *exercise.Repo, logentryRepo *logentry.Repo, sessionRepo *session.Repo) *Service {
	return &Service{exerciseRepo: exerciseRepo, logentryRepo: logentryRepo, sessionRepo: sessionRepo}
}

// History fetches log history for an exercise, resolved by category and variant name.
func (s *Service) History(ctx context.Context, userID uuid.UUID, categoryName, variantName string, limit int) ([]HistoryEntry, *exercise.Variant, error) {
	variant, err := s.exerciseRepo.Resolve(ctx, userID, categoryName, variantName)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve exercise: %w", err)
	}
	if variant == nil {
		return nil, nil, nil
	}
	entries, err := s.logentryRepo.ListByUserAndVariant(ctx, userID, variant.ID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("list entries: %w", err)
	}
	out := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		sets := make([]SetSummary, len(e.Sets))
		for j, set := range e.Sets {
			sets[j] = SetSummary{Weight: set.Weight, Reps: set.Reps, SetType: set.SetType}
		}
		sessionDate := ""
		if sess, _ := s.sessionRepo.GetByID(ctx, e.SessionID); sess != nil {
			sessionDate = sess.Date.Format("2006-01-02")
		}
		out[i] = HistoryEntry{
			SessionDate: sessionDate,
			RawSpeech:   e.RawSpeech,
			Sets:        sets,
			CreatedAt:   e.CreatedAt,
		}
	}
	return out, variant, nil
}
