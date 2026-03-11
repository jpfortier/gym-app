package query

import (
	"context"
	"fmt"
	"math"

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

// Query executes the Query DSL with scopes and metrics.
func (s *Service) Query(ctx context.Context, userID uuid.UUID, params QueryParams) (*QueryResult, error) {
	if params.Category == "" {
		return nil, fmt.Errorf("category required")
	}
	if params.Variant == "" {
		params.Variant = "standard"
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 50 {
		params.Limit = 50
	}

	variant, err := s.exerciseRepo.Resolve(ctx, userID, params.Category, params.Variant)
	if err != nil {
		return nil, fmt.Errorf("resolve exercise: %w", err)
	}
	if variant == nil {
		return nil, nil
	}

	entries, err := s.logentryRepo.ListByUserAndVariantWithDateRange(ctx, userID, variant.ID, params.FromDate, params.ToDate, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	cat, _ := s.exerciseRepo.GetCategoryByID(ctx, variant.CategoryID)
	catName := ""
	if cat != nil {
		catName = cat.Name
	}

	result := &QueryResult{
		ExerciseName: catName,
		VariantName:  variant.Name,
	}

	// Build HistoryEntry slice
	history := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		sets := make([]SetSummary, len(e.Sets))
		for j, set := range e.Sets {
			sets[j] = SetSummary{Weight: set.Weight, Reps: set.Reps, SetType: set.SetType}
		}
		sessionDate := ""
		if sess, _ := s.sessionRepo.GetByID(ctx, e.SessionID); sess != nil {
			sessionDate = sess.Date.Format("2006-01-02")
		}
		history[i] = HistoryEntry{SessionDate: sessionDate, RawSpeech: e.RawSpeech, Sets: sets, CreatedAt: e.CreatedAt}
	}
	result.Entries = history

	// Apply scope
	switch params.Scope {
	case "most_recent":
		if len(history) > 0 {
			result.Entries = history[:1]
		}
	case "best":
		bestIdx := findBestEntry(history)
		if bestIdx >= 0 {
			result.Entries = []HistoryEntry{history[bestIdx]}
		}
	case "aggregate":
		result.CountSets, result.CountSessions, result.TotalVolume = computeAggregate(history)
		result.Entries = nil
	default:
		// recent, session_detail, trend: use all
	}

	// Apply metric (overrides scope for single-value queries)
	switch params.Metric {
	case "max_weight":
		if v := maxWeightFromEntries(history); v != nil {
			result.Metric = "max_weight"
			result.Value = v
		}
	case "latest_weight":
		if v := latestWeightFromEntries(history); v != nil {
			result.Metric = "latest_weight"
			result.Value = v
		}
	case "max_reps":
		if v := maxRepsFromEntries(history); v != nil {
			result.Metric = "max_reps"
			result.Value = v
		}
	case "estimated_1rm":
		if v := estimated1RMFromEntries(history); v != nil {
			result.Metric = "estimated_1rm"
			result.Value = v
		}
	case "count_sets":
		cs, _, _ := computeAggregate(history)
		v := float64(cs)
		result.Metric = "count_sets"
		result.Value = &v
	case "count_sessions":
		_, sess, _ := computeAggregate(history)
		v := float64(sess)
		result.Metric = "count_sessions"
		result.Value = &v
	case "total_volume":
		_, _, vol := computeAggregate(history)
		result.Metric = "total_volume"
		result.Value = &vol
	}

	return result, nil
}

func findBestEntry(entries []HistoryEntry) int {
	if len(entries) == 0 {
		return -1
	}
	bestIdx := 0
	bestWeight := 0.0
	for i, e := range entries {
		for _, s := range e.Sets {
			if s.Weight != nil && *s.Weight > bestWeight {
				bestWeight = *s.Weight
				bestIdx = i
			}
		}
	}
	return bestIdx
}

func computeAggregate(entries []HistoryEntry) (countSets, countSessions int, totalVolume float64) {
	seenSessions := make(map[string]bool)
	for _, e := range entries {
		seenSessions[e.SessionDate] = true
		for _, s := range e.Sets {
			countSets++
			if s.Weight != nil {
				reps := s.Reps
				if reps == 0 {
					reps = 1
				}
				totalVolume += *s.Weight * float64(reps)
			}
		}
	}
	return countSets, len(seenSessions), totalVolume
}

func maxWeightFromEntries(entries []HistoryEntry) *float64 {
	var max *float64
	for _, e := range entries {
		for _, s := range e.Sets {
			if s.Weight != nil && (max == nil || *s.Weight > *max) {
				m := *s.Weight
				max = &m
			}
		}
	}
	return max
}

func latestWeightFromEntries(entries []HistoryEntry) *float64 {
	if len(entries) == 0 {
		return nil
	}
	sets := entries[0].Sets
	for i := len(sets) - 1; i >= 0; i-- {
		if sets[i].Weight != nil {
			w := *sets[i].Weight
			return &w
		}
	}
	return nil
}

func maxRepsFromEntries(entries []HistoryEntry) *float64 {
	var max *float64
	for _, e := range entries {
		for _, s := range e.Sets {
			if s.Reps > 0 {
				m := float64(s.Reps)
				if max == nil || m > *max {
					max = &m
				}
			}
		}
	}
	return max
}

// Epley formula: 1RM = weight * (1 + reps/30)
func estimated1RMFromEntries(entries []HistoryEntry) *float64 {
	var best *float64
	for _, e := range entries {
		for _, s := range e.Sets {
			if s.Weight != nil && s.Reps > 0 {
				est := *s.Weight * (1 + float64(s.Reps)/30)
				if best == nil || est > *best {
					best = &est
				}
			}
		}
	}
	if best != nil {
		rounded := math.Round(*best*10) / 10
		return &rounded
	}
	return nil
}

// History fetches log history for an exercise, resolved by category and variant name.
// fromDate, toDate are optional (YYYY-MM-DD). Empty means no filter.
func (s *Service) History(ctx context.Context, userID uuid.UUID, categoryName, variantName string, fromDate, toDate string, limit int) ([]HistoryEntry, *exercise.Variant, error) {
	variant, err := s.exerciseRepo.Resolve(ctx, userID, categoryName, variantName)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve exercise: %w", err)
	}
	if variant == nil {
		return nil, nil, nil
	}
	entries, err := s.logentryRepo.ListByUserAndVariantWithDateRange(ctx, userID, variant.ID, fromDate, toDate, limit)
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
