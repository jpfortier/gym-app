package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
	"github.com/jpfortier/gym-app/internal/user"
	"github.com/jpfortier/gym-app/internal/workoutcontext"
)

// Response is the unified chat response.
type Response struct {
	Intent   string        `json:"intent"`
	Message  string        `json:"message,omitempty"`
	Entries  []LogResult   `json:"entries,omitempty"`
	History  *HistoryResult `json:"history,omitempty"`
	PRs      []PRResult    `json:"prs,omitempty"`
	NeedsConfirmation bool   `json:"needs_confirmation,omitempty"`
}

type LogResult struct {
	ExerciseName string `json:"exercise_name"`
	VariantName  string `json:"variant_name"`
	SessionDate  string `json:"session_date"`
	EntryID      string `json:"entry_id"`
}

type HistoryResult struct {
	ExerciseName string        `json:"exercise_name"`
	VariantName  string        `json:"variant_name"`
	Entries      []interface{} `json:"entries"`
}

type PRResult struct {
	ID       string  `json:"id"`
	Exercise string  `json:"exercise_name"`
	Variant  string  `json:"variant_name"`
	Weight   float64 `json:"weight"`
	Reps     *int    `json:"reps,omitempty"`
	PRType   string  `json:"pr_type"`
}

// Config holds dependencies for the chat service.
type Config struct {
	Client           *ai.Client
	Parser           ai.Parser
	UserRepo         *user.Repo
	NameHandler      *name.Handler
	SessionSvc       *session.Service
	SessionRepo      *session.Repo
	LogentrySvc      *logentry.Service
	LogentryRepo     *logentry.Repo
	ExerciseSvc      *exercise.Service
	ExerciseRepo     *exercise.Repo
	QuerySvc         *query.Service
	CorrectionSvc    *correction.Service
	PrSvc            *pr.Service
	PrRepo           *pr.Repo
	NotesRepo        *notes.Repo
	ChatMessagesRepo *chatmessages.Repo
	R2               *storage.R2
}

type Service struct {
	client           *ai.Client
	parser           ai.Parser
	userRepo         *user.Repo
	nameHandler      *name.Handler
	sessionSvc       *session.Service
	logentrySvc      *logentry.Service
	logentryRepo     *logentry.Repo
	exerciseSvc      *exercise.Service
	exerciseRepo     *exercise.Repo
	querySvc         *query.Service
	correctionSvc    *correction.Service
	prSvc            *pr.Service
	prRepo           *pr.Repo
	notesRepo        *notes.Repo
	chatMessagesRepo *chatmessages.Repo
	r2               *storage.R2
	workoutCtxBuilder *workoutcontext.Builder
}

func NewService(cfg Config) *Service {
	var wcBuilder *workoutcontext.Builder
	if cfg.SessionRepo != nil && cfg.LogentryRepo != nil && cfg.ExerciseRepo != nil {
		wcBuilder = workoutcontext.NewBuilder(cfg.SessionRepo, cfg.LogentryRepo, cfg.ExerciseRepo)
	}
	return &Service{
		client:            cfg.Client,
		parser:            cfg.Parser,
		userRepo:          cfg.UserRepo,
		nameHandler:       cfg.NameHandler,
		sessionSvc:        cfg.SessionSvc,
		logentrySvc:       cfg.LogentrySvc,
		logentryRepo:      cfg.LogentryRepo,
		exerciseSvc:       cfg.ExerciseSvc,
		exerciseRepo:      cfg.ExerciseRepo,
		querySvc:          cfg.QuerySvc,
		correctionSvc:     cfg.CorrectionSvc,
		prSvc:             cfg.PrSvc,
		prRepo:            cfg.PrRepo,
		notesRepo:         cfg.NotesRepo,
		chatMessagesRepo:  cfg.ChatMessagesRepo,
		r2:                cfg.R2,
		workoutCtxBuilder: wcBuilder,
	}
}

// Process handles text or audio and returns the response.
// audioFormat is optional (e.g. "m4a", "webm"); used when audioBase64 is provided.
func (s *Service) Process(ctx context.Context, u *user.User, text string, audioBase64 string, audioFormat string) (*Response, error) {
	userID := u.ID
	if text == "" && audioBase64 != "" {
		var err error
		text, err = s.client.Transcribe(ctx, userID, audioBase64, audioFormat)
		if err != nil {
			return nil, fmt.Errorf("transcribe: %w", err)
		}
	}
	if strings.TrimSpace(text) == "" {
		return &Response{Intent: "unknown", Message: "No input received."}, nil
	}
	recent := s.loadRecentMessages(ctx, userID)
	var workoutCtxStr string
	if s.workoutCtxBuilder != nil {
		if wc, err := s.workoutCtxBuilder.Build(ctx, userID); err == nil {
			workoutCtxStr = wc.FormatForLLM()
		}
	}
	intent, err := s.parser.Parse(ctx, userID, text, recent, workoutCtxStr, u.Name)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if s.requiresConfirmation(intent) {
		msg := "I need a bit more detail to be sure."
		if intent.UIText != nil && intent.UIText.Preview != "" {
			msg = intent.UIText.Preview + " Can you clarify which one you mean?"
		} else if len(intent.Ambiguities) > 0 {
			msg = "I'm not sure which entry you mean. Can you be more specific?"
		}
		return &Response{Intent: intent.Intent, Message: msg, NeedsConfirmation: true}, nil
	}
	var resp *Response
	var handleErr error
	switch intent.Intent {
	case "log":
		resp, handleErr = s.handleLog(ctx, userID, intent)
	case "query":
		resp, handleErr = s.handleQuery(ctx, userID, intent)
	case "correction":
		resp, handleErr = s.handleCorrection(ctx, userID, intent)
	case "remove":
		resp, handleErr = s.handleRemove(ctx, userID, intent)
	case "restore":
		resp, handleErr = s.handleRestore(ctx, userID, intent)
	case "note":
		resp, handleErr = s.handleNote(ctx, userID, intent)
	case "set_name", "update_name":
		resp, handleErr = s.handleName(ctx, u, intent)
	default:
		resp = &Response{Intent: "unknown", Message: "I didn't understand. Try logging a workout, asking about your history, correcting a previous entry, or removing something."}
	}
	if handleErr != nil {
		return nil, handleErr
	}
	s.appendMessages(ctx, userID, text, resp, intent)
	return resp, nil
}

const contextWindowSize = 6

func (s *Service) requiresConfirmation(intent *ai.ParsedIntent) bool {
	if intent == nil || len(intent.Ambiguities) == 0 {
		return false
	}
	switch intent.Intent {
	case "correction", "remove":
		return true
	default:
		return false
	}
}

func (s *Service) loadRecentMessages(ctx context.Context, userID uuid.UUID) []ai.ChatMessage {
	if s.chatMessagesRepo == nil {
		return nil
	}
	msgs, err := s.chatMessagesRepo.ListRecent(ctx, userID, contextWindowSize)
	if err != nil {
		return nil
	}
	out := make([]ai.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = ai.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return out
}

func (s *Service) appendMessages(ctx context.Context, userID uuid.UUID, userText string, resp *Response, intent *ai.ParsedIntent) {
	if s.chatMessagesRepo == nil {
		return
	}
	summary := buildAssistantSummary(resp, intent)
	_ = s.chatMessagesRepo.Append(ctx, userID, "user", userText)
	_ = s.chatMessagesRepo.Append(ctx, userID, "assistant", summary)
}

func buildAssistantSummary(r *Response, intent *ai.ParsedIntent) string {
	switch r.Intent {
	case "log":
		if intent != nil && len(intent.Exercises) > 0 {
			var parts []string
			for _, ex := range intent.Exercises {
				name := strings.ToLower(ex.ExerciseName)
				if ex.VariantName != "" && ex.VariantName != "standard" {
					name += " " + strings.ToLower(ex.VariantName)
				}
				for _, set := range ex.Sets {
					if set.Weight != nil {
						parts = append(parts, fmt.Sprintf("%s %.0f×%d", name, *set.Weight, set.Reps))
					} else {
						parts = append(parts, fmt.Sprintf("%s %d reps", name, set.Reps))
					}
				}
			}
			if len(parts) > 0 {
				return fmt.Sprintf("Logged %s.", strings.Join(parts, ", "))
			}
		}
		if len(r.Entries) > 0 {
			names := make([]string, len(r.Entries))
			for i, e := range r.Entries {
				names[i] = fmt.Sprintf("%s %s", e.ExerciseName, e.VariantName)
			}
			return fmt.Sprintf("Logged %s.", strings.Join(names, ", "))
		}
		return r.Message
	case "query":
		if r.History != nil && r.History.ExerciseName != "" {
			return fmt.Sprintf("Here's your %s %s history.", r.History.ExerciseName, r.History.VariantName)
		}
		return "No history found."
	case "correction":
		return "Corrected."
	case "remove":
		return "Removed."
	case "restore":
		return "Brought back."
	case "note":
		return "Noted."
	case "set_name", "update_name":
		return r.Message
	default:
		return r.Message
	}
}

func (s *Service) handleLog(ctx context.Context, userID uuid.UUID, intent *ai.ParsedIntent) (*Response, error) {
	date := intent.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	var results []LogResult
	var allPRs []PRResult
	for _, ex := range intent.Exercises {
		variant, err := s.exerciseSvc.ResolveOrCreate(ctx, userID, strings.ToLower(ex.ExerciseName), strings.ToLower(ex.VariantName))
		if err != nil {
			return nil, err
		}
		if variant == nil {
			return nil, fmt.Errorf("exercise not found: %s / %s", ex.ExerciseName, ex.VariantName)
		}
		sets := make([]logentry.SetInput, len(ex.Sets))
		for i, ps := range ex.Sets {
			so := ps.SetOrder
			if so == 0 {
				so = i + 1
			}
			sets[i] = logentry.SetInput{Weight: ps.Weight, Reps: ps.Reps, SetOrder: so, SetType: ps.SetType}
		}
		entry, err := s.logentrySvc.CreateLogEntry(ctx, userID, date, variant.ID, ex.RawSpeech, ex.Notes, sets)
		if err != nil {
			return nil, err
		}
		full, _ := s.logentryRepo.GetByID(ctx, entry.ID)
		if full != nil {
			prs, _ := s.prSvc.CheckAndCreatePRs(ctx, userID, full)
			for _, p := range prs {
				cat, _ := s.exerciseRepo.GetCategoryByID(ctx, variant.CategoryID)
				catName := ""
				if cat != nil {
					catName = cat.Name
				}
				allPRs = append(allPRs, PRResult{ID: p.ID.String(), Exercise: catName, Variant: variant.Name, Weight: p.Weight, Reps: p.Reps, PRType: p.PRType})
				s.generateAndUploadPRImage(ctx, userID, p, catName)
			}
		}
		cat, _ := s.exerciseRepo.GetCategoryByID(ctx, variant.CategoryID)
		catName := ""
		if cat != nil {
			catName = cat.Name
		}
		results = append(results, LogResult{ExerciseName: catName, VariantName: variant.Name, SessionDate: date, EntryID: entry.ID.String()})
	}
	msg := "Logged."
	if len(allPRs) > 0 {
		msg = fmt.Sprintf("Logged. %d new PR(s)!", len(allPRs))
	}
	return &Response{Intent: "log", Message: msg, Entries: results, PRs: allPRs}, nil
}

func (s *Service) handleQuery(ctx context.Context, userID uuid.UUID, intent *ai.ParsedIntent) (*Response, error) {
	cat := intent.Category
	if cat == "" {
		cat = "bench press"
	}
	variant := intent.Variant
	if variant == "" {
		variant = "standard"
	}
	entries, v, err := s.querySvc.History(ctx, userID, cat, variant, "", "", 20)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return &Response{Intent: "query", History: &HistoryResult{ExerciseName: "", VariantName: "", Entries: []interface{}{}}}, nil
	}
	catObj, _ := s.exerciseRepo.GetCategoryByID(ctx, v.CategoryID)
	catName := ""
	if catObj != nil {
		catName = catObj.Name
	}
	out := make([]interface{}, len(entries))
	for i, e := range entries {
		sets := make([]map[string]interface{}, len(e.Sets))
		for j, set := range e.Sets {
			sets[j] = map[string]interface{}{"weight": set.Weight, "reps": set.Reps, "set_type": set.SetType}
		}
		out[i] = map[string]interface{}{
			"session_date": e.SessionDate,
			"raw_speech":   e.RawSpeech,
			"sets":         sets,
			"created_at":   e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return &Response{
		Intent:  "query",
		History: &HistoryResult{ExerciseName: catName, VariantName: v.Name, Entries: out},
	}, nil
}

func (s *Service) handleRemove(ctx context.Context, userID uuid.UUID, intent *ai.ParsedIntent) (*Response, error) {
	date := intent.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	var entry *logentry.LogEntry
	if intent.Category != "" && intent.Variant != "" {
		v, err := s.exerciseRepo.Resolve(ctx, userID, intent.Category, intent.Variant)
		if err != nil || v == nil {
			return &Response{Intent: "remove", Message: "Couldn't find that exercise."}, nil
		}
		entries, err := s.logentryRepo.ListByUserAndVariantWithDateRange(ctx, userID, v.ID, date, date, 1)
		if err != nil || len(entries) == 0 {
			return &Response{Intent: "remove", Message: "No matching entry found to remove."}, nil
		}
		entry = entries[0]
	} else {
		var err error
		entry, err = s.logentryRepo.GetMostRecentEntryForUser(ctx, userID, date)
		if err != nil || entry == nil {
			return &Response{Intent: "remove", Message: "No entry found to remove."}, nil
		}
	}
	ok, err := s.logentryRepo.DisableEntry(ctx, entry.ID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &Response{Intent: "remove", Message: "Couldn't remove that entry."}, nil
	}
	return &Response{Intent: "remove", Message: "Removed."}, nil
}

func (s *Service) handleRestore(ctx context.Context, userID uuid.UUID, intent *ai.ParsedIntent) (*Response, error) {
	date := intent.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	entry, err := s.logentryRepo.GetMostRecentDisabledEntryForUser(ctx, userID, date)
	if err != nil || entry == nil {
		return &Response{Intent: "restore", Message: "Nothing to bring back."}, nil
	}
	ok, err := s.logentryRepo.RestoreEntry(ctx, entry.ID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &Response{Intent: "restore", Message: "Couldn't restore that entry."}, nil
	}
	return &Response{Intent: "restore", Message: "Brought back."}, nil
}

func (s *Service) handleNote(ctx context.Context, userID uuid.UUID, intent *ai.ParsedIntent) (*Response, error) {
	content := strings.TrimSpace(intent.NoteContent)
	if content == "" {
		return &Response{Intent: "note", Message: "What should I remember?"}, nil
	}
	var categoryID, variantID *uuid.UUID
	if intent.Category != "" && intent.Variant != "" {
		v, err := s.exerciseRepo.Resolve(ctx, userID, intent.Category, intent.Variant)
		if err != nil || v == nil {
			v, err = s.exerciseSvc.ResolveOrCreate(ctx, userID, intent.Category, intent.Variant)
			if err != nil || v == nil {
				return &Response{Intent: "note", Message: "Couldn't find that exercise."}, nil
			}
		}
		cat, _ := s.exerciseRepo.GetCategoryByID(ctx, v.CategoryID)
		if cat != nil {
			categoryID = &cat.ID
		}
		variantID = &v.ID // variant-scoped note
	}
	_, err := s.notesRepo.Create(ctx, userID, categoryID, variantID, content)
	if err != nil {
		return nil, err
	}
	return &Response{Intent: "note", Message: "Noted."}, nil
}

func (s *Service) handleName(ctx context.Context, u *user.User, intent *ai.ParsedIntent) (*Response, error) {
	if s.nameHandler == nil || s.userRepo == nil {
		return &Response{Intent: intent.Intent, Message: "Name feature not available."}, nil
	}
	rawName := strings.TrimSpace(intent.Name)
	if rawName == "" {
		return &Response{Intent: intent.Intent, Message: "What should I call you?"}, nil
	}
	isRename := intent.Intent == "update_name"
	storedName, msg, err := s.nameHandler.Process(ctx, u.ID, rawName, isRename)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdateName(ctx, u.ID, storedName); err != nil {
		return nil, err
	}
	return &Response{Intent: intent.Intent, Message: msg}, nil
}

func (s *Service) handleCorrection(ctx context.Context, userID uuid.UUID, intent *ai.ParsedIntent) (*Response, error) {
	cat := intent.Category
	if cat == "" {
		cat = "bench press"
	}
	variant := intent.Variant
	if variant == "" {
		variant = "standard"
	}
	if err := s.correctionSvc.Apply(ctx, userID, cat, variant, intent.Changes); err != nil {
		return nil, err
	}
	return &Response{Intent: "correction", Message: "Corrected."}, nil
}

func (s *Service) generateAndUploadPRImage(ctx context.Context, userID uuid.UUID, p *pr.PersonalRecord, exerciseName string) {
	if s.r2 == nil {
		return
	}
	img, err := s.client.GeneratePRImage(ctx, userID, exerciseName, p.Weight, p.Reps, p.PRType)
	if err != nil {
		slog.Warn("pr image generation failed", "pr_id", p.ID, "err", err)
		return
	}
	if len(img) == 0 {
		return
	}
	if err := s.r2.PutPRImageBytes(ctx, userID, p.ID, img); err != nil {
		slog.Warn("pr image upload failed", "pr_id", p.ID, "err", err)
		return
	}
	path := fmt.Sprintf("pr/%s/%s.png", userID.String(), p.ID.String())
	if err := s.prRepo.UpdateImageURL(ctx, p.ID, path); err != nil {
		slog.Warn("pr image_url update failed", "pr_id", p.ID, "err", err)
	}
}
