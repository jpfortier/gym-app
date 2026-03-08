package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
)

// Response is the unified chat response.
type Response struct {
	Intent   string        `json:"intent"`
	Message  string        `json:"message,omitempty"`
	Entries  []LogResult   `json:"entries,omitempty"`
	History  *HistoryResult `json:"history,omitempty"`
	PRs      []PRResult    `json:"prs,omitempty"`
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

type Service struct {
	client        *ai.Client
	parser        ai.Parser
	sessionSvc    *session.Service
	logentrySvc   *logentry.Service
	logentryRepo  *logentry.Repo
	exerciseRepo  *exercise.Repo
	querySvc      *query.Service
	correctionSvc  *correction.Service
	prSvc         *pr.Service
	prRepo        *pr.Repo
	r2            *storage.R2
}

func NewService(
	client *ai.Client,
	parser ai.Parser,
	sessionSvc *session.Service,
	logentrySvc *logentry.Service,
	logentryRepo *logentry.Repo,
	exerciseRepo *exercise.Repo,
	querySvc *query.Service,
	correctionSvc *correction.Service,
	prSvc *pr.Service,
	prRepo *pr.Repo,
	r2 *storage.R2,
) *Service {
	return &Service{
		client:        client,
		parser:        parser,
		sessionSvc:   sessionSvc,
		logentrySvc:  logentrySvc,
		logentryRepo: logentryRepo,
		exerciseRepo: exerciseRepo,
		querySvc:     querySvc,
		correctionSvc: correctionSvc,
		prSvc:        prSvc,
		prRepo:       prRepo,
		r2:           r2,
	}
}

// Process handles text or audio and returns the response.
func (s *Service) Process(ctx context.Context, userID uuid.UUID, text string, audioBase64 string) (*Response, error) {
	if text == "" && audioBase64 != "" {
		var err error
		text, err = s.client.Transcribe(ctx, userID, audioBase64)
		if err != nil {
			return nil, fmt.Errorf("transcribe: %w", err)
		}
	}
	if strings.TrimSpace(text) == "" {
		return &Response{Intent: "unknown", Message: "No input received."}, nil
	}
	intent, err := s.parser.Parse(ctx, userID, text)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	switch intent.Intent {
	case "log":
		return s.handleLog(ctx, userID, intent)
	case "query":
		return s.handleQuery(ctx, userID, intent)
	case "correction":
		return s.handleCorrection(ctx, userID, intent)
	default:
		return &Response{Intent: "unknown", Message: "I didn't understand. Try logging a workout, asking about your history, or correcting a previous entry."}, nil
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
		variant, err := s.exerciseRepo.Resolve(ctx, userID, strings.ToLower(ex.ExerciseName), strings.ToLower(ex.VariantName))
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
