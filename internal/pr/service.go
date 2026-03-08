package pr

import (
	"context"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/logentry"
)

// Service detects PRs and creates records.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CheckAndCreatePRs checks each set in the entry for new PRs and creates records.
// Call after creating a log entry. entry must have Sets populated (e.g. from GetByID).
func (s *Service) CheckAndCreatePRs(ctx context.Context, userID uuid.UUID, entry *logentry.LogEntry) ([]*PersonalRecord, error) {
	existing, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	byVariant := make(map[uuid.UUID][]*PersonalRecord)
	for _, p := range existing {
		byVariant[p.ExerciseVariantID] = append(byVariant[p.ExerciseVariantID], p)
	}
	var created []*PersonalRecord
	for _, set := range entry.Sets {
		pr, err := s.checkSet(ctx, userID, entry.ExerciseVariantID, &set, byVariant[entry.ExerciseVariantID])
		if err != nil {
			return nil, err
		}
		if pr != nil {
			if err := s.repo.Create(ctx, pr); err != nil {
				return nil, err
			}
			created = append(created, pr)
			byVariant[entry.ExerciseVariantID] = append(byVariant[entry.ExerciseVariantID], pr)
		}
	}
	return created, nil
}

func (s *Service) checkSet(ctx context.Context, userID, variantID uuid.UUID, set *logentry.LogEntrySet, existing []*PersonalRecord) (*PersonalRecord, error) {
	weight := 0.0
	if set.Weight != nil {
		weight = *set.Weight
	}
	reps := set.Reps

	if reps == 1 && weight > 0 {
		for _, p := range existing {
			if p.PRType == "one_rep_max" && p.Weight >= weight {
				return nil, nil
			}
		}
		return &PersonalRecord{
			UserID:            userID,
			ExerciseVariantID: variantID,
			PRType:            "one_rep_max",
			Weight:            weight,
			Reps:              &reps,
			LogEntrySetID:     &set.ID,
		}, nil
	}

	if weight > 0 && reps > 1 {
		for _, p := range existing {
			if p.PRType == "natural_set" && p.Weight > 0 && p.Weight >= weight && (p.Reps == nil || *p.Reps >= reps) {
				return nil, nil
			}
		}
		return &PersonalRecord{
			UserID:            userID,
			ExerciseVariantID: variantID,
			PRType:            "natural_set",
			Weight:            weight,
			Reps:              &reps,
			LogEntrySetID:     &set.ID,
		}, nil
	}

	if weight == 0 && reps > 0 {
		for _, p := range existing {
			if p.PRType == "natural_set" && p.Weight == 0 && p.Reps != nil && *p.Reps >= reps {
				return nil, nil
			}
		}
		return &PersonalRecord{
			UserID:            userID,
			ExerciseVariantID: variantID,
			PRType:            "natural_set",
			Weight:            0,
			Reps:              &reps,
			LogEntrySetID:     &set.ID,
		}, nil
	}
	return nil, nil
}
