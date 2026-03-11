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
		pr, celebrate, err := s.checkSet(ctx, userID, entry.ExerciseVariantID, &set, byVariant[entry.ExerciseVariantID])
		if err != nil {
			return nil, err
		}
		if pr != nil {
			if err := s.repo.Create(ctx, pr); err != nil {
				return nil, err
			}
			byVariant[entry.ExerciseVariantID] = append(byVariant[entry.ExerciseVariantID], pr)
			if celebrate {
				created = append(created, pr)
			}
		}
	}
	return created, nil
}

func (s *Service) checkSet(ctx context.Context, userID, variantID uuid.UUID, set *logentry.LogEntrySet, existing []*PersonalRecord) (*PersonalRecord, bool, error) {
	weight := 0.0
	if set.Weight != nil {
		weight = *set.Weight
	}
	reps := set.Reps

	if reps == 1 && weight > 0 {
		existing1RM := filterByType(existing, "one_rep_max", nil)
		if len(existing1RM) == 0 {
			return &PersonalRecord{
				UserID:            userID,
				ExerciseVariantID: variantID,
				PRType:            "one_rep_max",
				Weight:            weight,
				Reps:              &reps,
				LogEntrySetID:     &set.ID,
			}, false, nil
		}
		for _, p := range existing1RM {
			if p.Weight >= weight {
				return nil, false, nil
			}
		}
		return &PersonalRecord{
			UserID:            userID,
			ExerciseVariantID: variantID,
			PRType:            "one_rep_max",
			Weight:            weight,
			Reps:              &reps,
			LogEntrySetID:     &set.ID,
		}, true, nil
	}

	if weight > 0 && reps > 1 {
		existingWeighted := filterByType(existing, "natural_set", func(p *PersonalRecord) bool { return p.Weight > 0 })
		if len(existingWeighted) == 0 {
			return &PersonalRecord{
				UserID:            userID,
				ExerciseVariantID: variantID,
				PRType:            "natural_set",
				Weight:            weight,
				Reps:              &reps,
				LogEntrySetID:     &set.ID,
			}, false, nil
		}
		for _, p := range existingWeighted {
			if p.Weight >= weight && (p.Reps == nil || *p.Reps >= reps) {
				return nil, false, nil
			}
		}
		return &PersonalRecord{
			UserID:            userID,
			ExerciseVariantID: variantID,
			PRType:            "natural_set",
			Weight:            weight,
			Reps:              &reps,
			LogEntrySetID:     &set.ID,
		}, true, nil
	}

	if weight == 0 && reps > 0 {
		existingBodyweight := filterByType(existing, "natural_set", func(p *PersonalRecord) bool { return p.Weight == 0 })
		if len(existingBodyweight) == 0 {
			return &PersonalRecord{
				UserID:            userID,
				ExerciseVariantID: variantID,
				PRType:            "natural_set",
				Weight:            0,
				Reps:              &reps,
				LogEntrySetID:     &set.ID,
			}, false, nil
		}
		for _, p := range existingBodyweight {
			if p.Reps != nil && *p.Reps >= reps {
				return nil, false, nil
			}
		}
		return &PersonalRecord{
			UserID:            userID,
			ExerciseVariantID: variantID,
			PRType:            "natural_set",
			Weight:            0,
			Reps:              &reps,
			LogEntrySetID:     &set.ID,
		}, true, nil
	}
	return nil, false, nil
}

func filterByType(prs []*PersonalRecord, prType string, extra func(*PersonalRecord) bool) []*PersonalRecord {
	var out []*PersonalRecord
	for _, p := range prs {
		if p.PRType != prType {
			continue
		}
		if extra != nil && !extra(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}
