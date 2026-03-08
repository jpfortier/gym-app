package correction

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
)

type Service struct {
	logentryRepo *logentry.Repo
	exerciseRepo *exercise.Repo
}

func NewService(logentryRepo *logentry.Repo, exerciseRepo *exercise.Repo) *Service {
	return &Service{logentryRepo: logentryRepo, exerciseRepo: exerciseRepo}
}

// Apply finds the target entry and applies the correction.
// category/variant from LLM identify the exercise. We find the most recent entry and update the first set.
func (s *Service) Apply(ctx context.Context, userID uuid.UUID, category, variant string, changes *ai.ParsedCorrection) error {
	if changes == nil {
		return fmt.Errorf("no changes specified")
	}
	v, err := s.exerciseRepo.Resolve(ctx, userID, category, variant)
	if err != nil || v == nil {
		return fmt.Errorf("resolve exercise: %w", err)
	}
	entries, err := s.logentryRepo.ListByUserAndVariant(ctx, userID, v.ID, 1)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no matching entry found")
	}
	entry := entries[0]
	if len(entry.Sets) == 0 {
		return fmt.Errorf("entry has no sets")
	}
	setID := entry.Sets[0].ID
	if changes.Weight != nil || changes.Reps != nil {
		return s.logentryRepo.UpdateSet(ctx, setID, changes.Weight, changes.Reps)
	}
	return nil
}
