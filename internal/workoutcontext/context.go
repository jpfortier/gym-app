package workoutcontext

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
)

func sanitizeForYAML(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// WorkoutContext is the structured context sent to the LLM for interpretation.
type WorkoutContext struct {
	Today           string
	RefObjects      RefObjects
	ActiveSession   *ActiveSession
	RecentSessions  []RecentSession
	ExerciseAliases map[string]string
	UserDefaults    UserDefaults
}

type RefObjects struct {
	LastCreatedSet  string // e.g. "set_abc123" (set ID for reference)
	LastExercise    string // e.g. "barbell bench press"
	LastSession     string // e.g. "2026-03-08"
}

type ActiveSession struct {
	ID        string
	Date      string
	Exercises []ActiveExercise
}

type ActiveExercise struct {
	ID    string
	Name  string
	Sets  []ActiveSet
}

type ActiveSet struct {
	ID     string
	Weight *float64
	Reps   int
}

type RecentSession struct {
	Date      string
	Exercises []RecentExercise
}

type RecentExercise struct {
	Name string
	Sets []RecentSet
}

type RecentSet struct {
	Weight *float64
	Reps   int
}

type UserDefaults struct {
	WeightUnit string
}

// Builder builds workout context for a user.
type Builder struct {
	sessionRepo  *session.Repo
	logentryRepo *logentry.Repo
	exerciseRepo *exercise.Repo
}

const recentSessionsLimit = 8

func NewBuilder(sessionRepo *session.Repo, logentryRepo *logentry.Repo, exerciseRepo *exercise.Repo) *Builder {
	return &Builder{
		sessionRepo:  sessionRepo,
		logentryRepo: logentryRepo,
		exerciseRepo: exerciseRepo,
	}
}

// Build constructs the workout context for the given user.
func (b *Builder) Build(ctx context.Context, userID uuid.UUID) (*WorkoutContext, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	wc := &WorkoutContext{
		Today:           today,
		RefObjects:      RefObjects{},
		RecentSessions:  nil,
		ExerciseAliases: make(map[string]string),
		UserDefaults:    UserDefaults{WeightUnit: "lb"},
	}

	aliases, err := b.exerciseRepo.ListUserAliases(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	for _, a := range aliases {
		wc.ExerciseAliases[a.AliasKey] = a.Canonical
	}

	sessions, err := b.sessionRepo.ListByUser(ctx, userID, recentSessionsLimit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	// lastSetID: from today's session; if today empty, fall back to most recent session.
	var lastSetID string
	var lastExerciseName string
	var lastSessionDate string

	for i, sess := range sessions {
		entries, err := b.logentryRepo.ListBySession(ctx, sess.ID)
		if err != nil {
			return nil, fmt.Errorf("list entries for session %s: %w", sess.ID, err)
		}

		dateStr := sess.Date.Format("2006-01-02")
		rs := RecentSession{Date: dateStr, Exercises: nil}

		for _, e := range entries {
			v, _ := b.exerciseRepo.GetVariantByID(ctx, e.ExerciseVariantID)
			exName := ""
			if v != nil {
				cat, _ := b.exerciseRepo.GetCategoryByID(ctx, v.CategoryID)
				if cat != nil {
					exName = strings.ToLower(cat.Name) + " " + strings.ToLower(v.Name)
				}
			}

			if i == 0 && dateStr == today {
				ae := ActiveExercise{ID: e.ID.String(), Name: exName, Sets: nil}
				for _, set := range e.Sets {
					reps := set.Reps
					if reps == 0 && set.Weight != nil && *set.Weight > 0 {
						reps = 1
					}
					ae.Sets = append(ae.Sets, ActiveSet{
						ID:     set.ID.String(),
						Weight: set.Weight,
						Reps:   reps,
					})
					lastSetID = set.ID.String()
				}
				if exName != "" {
					lastExerciseName = exName
				}
				if wc.ActiveSession == nil {
					wc.ActiveSession = &ActiveSession{
						ID:        sess.ID.String(),
						Date:      dateStr,
						Exercises: nil,
					}
				}
				wc.ActiveSession.Exercises = append(wc.ActiveSession.Exercises, ae)
			} else {
				re := RecentExercise{Name: exName, Sets: nil}
				for _, set := range e.Sets {
					reps := set.Reps
					if reps == 0 && set.Weight != nil && *set.Weight > 0 {
						reps = 1
					}
					re.Sets = append(re.Sets, RecentSet{Weight: set.Weight, Reps: reps})
					if lastSetID == "" && i == 0 {
						lastSetID = set.ID.String()
					}
				}
				rs.Exercises = append(rs.Exercises, re)
			}
		}

		// lastSessionDate: prefer today or yesterday when present; else use most recent session.
		if lastSessionDate == "" && (dateStr == today || dateStr == yesterday) {
			lastSessionDate = dateStr
		}

		if i > 0 || dateStr != today {
			wc.RecentSessions = append(wc.RecentSessions, rs)
		}
	}

	if lastSessionDate == "" && len(sessions) > 0 {
		lastSessionDate = sessions[0].Date.Format("2006-01-02")
	}
	wc.RefObjects = RefObjects{
		LastCreatedSet: lastSetID,
		LastExercise:   lastExerciseName,
		LastSession:    lastSessionDate,
	}

	return wc, nil
}

// FormatForLLM returns a compact YAML-like string for the LLM prompt.
func (wc *WorkoutContext) FormatForLLM() string {
	var b strings.Builder
	b.WriteString("today: " + wc.Today + "\n\n")
	b.WriteString("REFERENCE_OBJECTS:\n")
	b.WriteString("  last_created_set: " + sanitizeForYAML(wc.RefObjects.LastCreatedSet) + "\n")
	b.WriteString("  last_exercise: " + sanitizeForYAML(wc.RefObjects.LastExercise) + "\n")
	b.WriteString("  last_session: " + sanitizeForYAML(wc.RefObjects.LastSession) + "\n\n")

	if wc.ActiveSession != nil {
		b.WriteString("active_session:\n")
		b.WriteString("  id: " + wc.ActiveSession.ID + "\n")
		b.WriteString("  date: " + wc.ActiveSession.Date + "\n")
		b.WriteString("  exercises:\n")
		for _, ex := range wc.ActiveSession.Exercises {
			b.WriteString("    - id: " + ex.ID + "\n")
			b.WriteString("      name: " + sanitizeForYAML(ex.Name) + "\n")
			b.WriteString("      sets:\n")
			for _, s := range ex.Sets {
				b.WriteString("        - id: " + s.ID + "\n")
				if s.Weight != nil {
					b.WriteString("          weight: " + strconv.FormatFloat(*s.Weight, 'f', -1, 64) + "\n")
				}
				b.WriteString("          reps: " + strconv.Itoa(s.Reps) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if len(wc.RecentSessions) > 0 {
		b.WriteString("recent_sessions:\n")
		for _, rs := range wc.RecentSessions {
			b.WriteString("  - date: " + rs.Date + "\n")
			b.WriteString("    exercises:\n")
			for _, ex := range rs.Exercises {
				b.WriteString("      - name: " + sanitizeForYAML(ex.Name) + "\n")
				b.WriteString("        sets:\n")
				for _, s := range ex.Sets {
					b.WriteString("          - weight: ")
					if s.Weight != nil {
						b.WriteString(strconv.FormatFloat(*s.Weight, 'f', -1, 64))
					} else {
						b.WriteString("null")
					}
					b.WriteString(", reps: " + strconv.Itoa(s.Reps) + "\n")
				}
			}
		}
		b.WriteString("\n")
	}

	if len(wc.ExerciseAliases) > 0 {
		b.WriteString("exercise_aliases:\n")
		keys := make([]string, 0, len(wc.ExerciseAliases))
		for k := range wc.ExerciseAliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := wc.ExerciseAliases[k]
			b.WriteString("  " + sanitizeForYAML(k) + " -> " + sanitizeForYAML(v) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("user_defaults:\n")
	b.WriteString("  weight_unit: " + wc.UserDefaults.WeightUnit + "\n")
	return b.String()
}
