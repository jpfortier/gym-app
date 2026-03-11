package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
	"github.com/jpfortier/gym-app/internal/workoutcontext"
)

// Executor runs Command DSL commands.
type Executor struct {
	sessionSvc   *session.Service
	logentrySvc  *logentry.Service
	logentryRepo *logentry.Repo
	exerciseSvc  *exercise.Service
	exerciseRepo *exercise.Repo
	userRepo     *user.Repo
	nameHandler  *name.Handler
	notesRepo    *notes.Repo
	prSvc        *pr.Service
}

// NewExecutor creates an Executor with the given dependencies.
func NewExecutor(
	sessionSvc *session.Service,
	logentrySvc *logentry.Service,
	logentryRepo *logentry.Repo,
	exerciseSvc *exercise.Service,
	exerciseRepo *exercise.Repo,
	userRepo *user.Repo,
	nameHandler *name.Handler,
	notesRepo *notes.Repo,
	prSvc *pr.Service,
) *Executor {
	return &Executor{
		sessionSvc:   sessionSvc,
		logentrySvc:  logentrySvc,
		logentryRepo: logentryRepo,
		exerciseSvc:  exerciseSvc,
		exerciseRepo: exerciseRepo,
		userRepo:     userRepo,
		nameHandler:  nameHandler,
		notesRepo:    notesRepo,
		prSvc:        prSvc,
	}
}

// refState tracks resolved IDs during execution for target_ref resolution.
type refState struct {
	lastCreatedSetID string
	lastEntryID      string
	lastExerciseName string
	lastSessionDate  string
}

func (e *Executor) Execute(ctx context.Context, userID uuid.UUID, commands []Command, wc *workoutcontext.WorkoutContext, defaultDate string) *ExecutionResult {
	if defaultDate == "" {
		defaultDate = time.Now().Format("2006-01-02")
	}
	refs := &refState{}
	if wc != nil {
		refs.lastCreatedSetID = wc.RefObjects.LastCreatedSet
		refs.lastExerciseName = wc.RefObjects.LastExercise
		refs.lastSessionDate = wc.RefObjects.LastSession
		if wc.ActiveSession != nil {
			refs.lastSessionDate = wc.ActiveSession.Date
			for _, ex := range wc.ActiveSession.Exercises {
				refs.lastEntryID = ex.ID
				for _, s := range ex.Sets {
					refs.lastCreatedSetID = s.ID
				}
			}
		}
	}
	if refs.lastSessionDate == "" {
		refs.lastSessionDate = defaultDate
	}

	result := &ExecutionResult{Success: true}
	for _, cmd := range commands {
		if err := e.executeOne(ctx, userID, &cmd, refs, defaultDate, result); err != nil {
			result.Success = false
			result.Error = err.Error()
			return result
		}
	}
	return result
}

func (e *Executor) executeOne(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState, defaultDate string, result *ExecutionResult) error {
	switch cmd.Type {
	case EnsureSession:
		return e.doEnsureSession(ctx, userID, cmd, refs, defaultDate)
	case CreateExerciseEntry:
		return e.doCreateExerciseEntry(ctx, userID, cmd, refs, defaultDate, result)
	case AppendSet:
		return e.doAppendSet(ctx, userID, cmd, refs, result)
	case UpdateSet:
		return e.doUpdateSet(ctx, userID, cmd, refs)
	case DeleteSet:
		return e.doDeleteSet(ctx, userID, cmd, refs)
	case DisableEntry:
		return e.doDisableEntry(ctx, userID, cmd, refs, defaultDate)
	case RestoreEntry:
		return e.doRestoreEntry(ctx, userID, cmd, refs, defaultDate, result)
	case SetName:
		return e.doSetName(ctx, userID, cmd, result)
	case UpdateName:
		return e.doUpdateName(ctx, userID, cmd, result)
	case CreateNote:
		return e.doCreateNote(ctx, userID, cmd)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

func (e *Executor) doEnsureSession(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState, defaultDate string) error {
	date := cmd.Date
	if date == "" {
		date = defaultDate
	}
	sess, err := e.sessionSvc.GetOrCreateForDate(ctx, userID, date)
	if err != nil {
		return fmt.Errorf("ensure session: %w", err)
	}
	refs.lastSessionDate = sess.Date.Format("2006-01-02")
	return nil
}

func (e *Executor) doCreateExerciseEntry(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState, defaultDate string, result *ExecutionResult) error {
	date := refs.lastSessionDate
	if date == "" {
		date = defaultDate
	}

	variant, err := e.exerciseSvc.ResolveOrCreate(ctx, userID, strings.ToLower(cmd.Exercise), strings.ToLower(cmd.Variant))
	if err != nil || variant == nil {
		return fmt.Errorf("resolve exercise %s/%s: %w", cmd.Exercise, cmd.Variant, err)
	}

	sets := make([]logentry.SetInput, len(cmd.Sets))
	for i, s := range cmd.Sets {
		so := s.SetOrder
		if so == 0 {
			so = i + 1
		}
		reps := s.Reps
		if reps == 0 && s.Weight != nil {
			reps = 1
		}
		sets[i] = logentry.SetInput{Weight: s.Weight, Reps: reps, SetOrder: so, SetType: s.SetType}
	}
	if len(sets) == 0 {
		sets = []logentry.SetInput{{Reps: 1, SetOrder: 1}}
	}

	entry, err := e.logentrySvc.CreateLogEntry(ctx, userID, date, variant.ID, cmd.RawSpeech, cmd.Notes, sets)
	if err != nil {
		return fmt.Errorf("create log entry: %w", err)
	}

	result.CreatedEntryIDs = append(result.CreatedEntryIDs, entry.ID.String())
	full, _ := e.logentryRepo.GetByID(ctx, entry.ID)
	if full != nil {
		for _, s := range full.Sets {
			result.CreatedSetIDs = append(result.CreatedSetIDs, s.ID.String())
			refs.lastCreatedSetID = s.ID.String()
		}
		refs.lastEntryID = entry.ID.String()
		cat, _ := e.exerciseRepo.GetCategoryByID(ctx, variant.CategoryID)
		if cat != nil {
			refs.lastExerciseName = strings.ToLower(cat.Name) + " " + strings.ToLower(variant.Name)
		}

		prs, _ := e.prSvc.CheckAndCreatePRs(ctx, userID, full)
		for _, p := range prs {
			cat, _ := e.exerciseRepo.GetCategoryByID(ctx, variant.CategoryID)
			catName := ""
			if cat != nil {
				catName = cat.Name
			}
			result.PRs = append(result.PRs, PRInfo{
				ID:       p.ID.String(),
				Exercise: catName,
				Variant:  variant.Name,
				Weight:   p.Weight,
				Reps:     p.Reps,
				PRType:   p.PRType,
			})
		}
	}
	return nil
}

func (e *Executor) doAppendSet(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState, result *ExecutionResult) error {
	entryID, err := e.resolveEntryID(ctx, userID, cmd.TargetRef, refs)
	if err != nil {
		return err
	}
	eid, _ := uuid.Parse(entryID)
	reps := 0
	if cmd.Reps != nil {
		reps = *cmd.Reps
	}
	if reps == 0 && cmd.Weight != nil {
		reps = 1
	}
	setID, err := e.logentryRepo.AppendSet(ctx, eid, cmd.Weight, reps, "")
	if err != nil {
		return fmt.Errorf("append set: %w", err)
	}
	result.CreatedSetIDs = append(result.CreatedSetIDs, setID.String())
	refs.lastCreatedSetID = setID.String()
	refs.lastEntryID = entryID
	return nil
}

func (e *Executor) doUpdateSet(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState) error {
	setID, err := e.resolveSetID(ctx, userID, cmd.TargetRef, refs)
	if err != nil {
		return err
	}
	sid, _ := uuid.Parse(setID)
	var weight *float64
	var reps *int
	if cmd.Changes != nil {
		weight = cmd.Changes.Weight
		reps = cmd.Changes.Reps
	} else {
		weight = cmd.Weight
		reps = cmd.Reps
	}
	return e.logentryRepo.UpdateSet(ctx, sid, weight, reps)
}

func (e *Executor) doDeleteSet(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState) error {
	setID, err := e.resolveSetID(ctx, userID, cmd.TargetRef, refs)
	if err != nil {
		return err
	}
	sid, _ := uuid.Parse(setID)
	return e.logentryRepo.DeleteSet(ctx, sid)
}

func (e *Executor) doDisableEntry(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState, defaultDate string) error {
	date := defaultDate
	if refs.lastSessionDate != "" {
		date = refs.lastSessionDate
	}
	var entry *logentry.LogEntry
	if cmd.Exercise != "" {
		variant := cmd.Variant
		if variant == "" {
			variant = "standard"
		}
		v, err := e.exerciseRepo.Resolve(ctx, userID, strings.ToLower(cmd.Exercise), strings.ToLower(variant))
		if err != nil || v == nil {
			return fmt.Errorf("resolve exercise: %w", err)
		}
		entries, err := e.logentryRepo.ListByUserAndVariantWithDateRange(ctx, userID, v.ID, date, date, 1)
		if err != nil || len(entries) == 0 {
			return fmt.Errorf("nothing to scratch")
		}
		entry = entries[0]
	} else {
		var err error
		entry, err = e.logentryRepo.GetMostRecentEntryForUser(ctx, userID, date)
		if err != nil || entry == nil {
			return fmt.Errorf("nothing to scratch")
		}
	}
	ok, err := e.logentryRepo.DisableEntry(ctx, entry.ID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("could not disable entry")
	}
	return nil
}

func (e *Executor) doRestoreEntry(ctx context.Context, userID uuid.UUID, cmd *Command, refs *refState, defaultDate string, result *ExecutionResult) error {
	date := defaultDate
	if refs.lastSessionDate != "" {
		date = refs.lastSessionDate
	}
	entry, err := e.logentryRepo.GetMostRecentDisabledEntryForUser(ctx, userID, date)
	if err != nil || entry == nil {
		return fmt.Errorf("nothing to restore")
	}
	ok, err := e.logentryRepo.RestoreEntry(ctx, entry.ID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("could not restore entry")
	}
	result.RestoredEntryID = entry.ID.String()
	refs.lastEntryID = entry.ID.String()
	if len(entry.Sets) > 0 {
		refs.lastCreatedSetID = entry.Sets[len(entry.Sets)-1].ID.String()
	}
	return nil
}

func (e *Executor) doSetName(ctx context.Context, userID uuid.UUID, cmd *Command, result *ExecutionResult) error {
	raw := strings.TrimSpace(cmd.Name)
	if raw == "" {
		return fmt.Errorf("name required")
	}
	stored, _, err := e.nameHandler.Process(ctx, userID, raw, false)
	if err != nil {
		return fmt.Errorf("process name: %w", err)
	}
	if err := e.userRepo.UpdateName(ctx, userID, stored); err != nil {
		return fmt.Errorf("update name: %w", err)
	}
	result.StoredName = stored
	return nil
}

func (e *Executor) doUpdateName(ctx context.Context, userID uuid.UUID, cmd *Command, result *ExecutionResult) error {
	raw := strings.TrimSpace(cmd.Name)
	if raw == "" {
		return fmt.Errorf("name required")
	}
	if err := e.userRepo.UpdateName(ctx, userID, raw); err != nil {
		return fmt.Errorf("update name: %w", err)
	}
	result.StoredName = raw
	return nil
}

func (e *Executor) doCreateNote(ctx context.Context, userID uuid.UUID, cmd *Command) error {
	content := strings.TrimSpace(cmd.Content)
	if content == "" {
		return fmt.Errorf("note content required")
	}
	var categoryID, variantID *uuid.UUID
	if cmd.Category != "" {
		variant := cmd.Variant
		if variant == "" {
			variant = "standard"
		}
		v, err := e.exerciseRepo.Resolve(ctx, userID, strings.ToLower(cmd.Category), strings.ToLower(variant))
		if err != nil || v == nil {
			v, err = e.exerciseSvc.ResolveOrCreate(ctx, userID, strings.ToLower(cmd.Category), strings.ToLower(variant))
			if err != nil || v == nil {
				return fmt.Errorf("resolve exercise for note: %w", err)
			}
		}
		cat, _ := e.exerciseRepo.GetCategoryByID(ctx, v.CategoryID)
		if cat != nil {
			categoryID = &cat.ID
		}
		variantID = &v.ID
	}
	_, err := e.notesRepo.Create(ctx, userID, categoryID, variantID, content)
	return err
}

func (e *Executor) resolveSetID(ctx context.Context, userID uuid.UUID, targetRef string, refs *refState) (string, error) {
	targetRef = strings.TrimSpace(strings.ToLower(targetRef))
	if targetRef == "" || targetRef == "last_created_set" || targetRef == "last" {
		if refs.lastCreatedSetID != "" {
			return refs.lastCreatedSetID, nil
		}
		return "", fmt.Errorf("target_ref last_created_set not found in context")
	}
	if _, err := uuid.Parse(targetRef); err == nil {
		return targetRef, nil
	}
	return "", fmt.Errorf("target_ref %q could not be resolved", targetRef)
}

func (e *Executor) resolveEntryID(ctx context.Context, userID uuid.UUID, targetRef string, refs *refState) (string, error) {
	targetRef = strings.TrimSpace(strings.ToLower(targetRef))
	if targetRef == "" || targetRef == "last_created_set" || targetRef == "last" {
		if refs.lastCreatedSetID != "" {
			sid, err := uuid.Parse(refs.lastCreatedSetID)
			if err == nil {
				entryID, err := e.logentryRepo.GetEntryIDBySetID(ctx, sid)
				if err == nil && entryID != uuid.Nil {
					return entryID.String(), nil
				}
			}
		}
		if refs.lastEntryID != "" {
			return refs.lastEntryID, nil
		}
		return "", fmt.Errorf("target_ref last_created_set not found in context")
	}
	if targetRef == "last_exercise" && refs.lastExerciseName != "" {
		parts := strings.SplitN(refs.lastExerciseName, " ", 2)
		cat := parts[0]
		variant := "standard"
		if len(parts) > 1 {
			variant = parts[1]
		}
		v, err := e.exerciseRepo.Resolve(ctx, userID, cat, variant)
		if err != nil || v == nil {
			return "", fmt.Errorf("resolve last_exercise: %w", err)
		}
		entries, err := e.logentryRepo.ListByUserAndVariantWithDateRange(ctx, userID, v.ID, refs.lastSessionDate, refs.lastSessionDate, 1)
		if err != nil || len(entries) == 0 {
			return "", fmt.Errorf("no entry for last_exercise")
		}
		refs.lastEntryID = entries[0].ID.String()
		return entries[0].ID.String(), nil
	}
	if _, err := uuid.Parse(targetRef); err == nil {
		return targetRef, nil
	}
	return "", fmt.Errorf("target_ref %q could not be resolved", targetRef)
}

