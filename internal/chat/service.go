package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/command"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
	"github.com/jpfortier/gym-app/internal/user"
	"github.com/jpfortier/gym-app/internal/workoutcontext"
)

// Response is the unified chat response. Message may contain Markdown.
type Response struct {
	Message string         `json:"message,omitempty"`
	Entries []LogResult    `json:"entries,omitempty"`
	History *HistoryResult `json:"history,omitempty"`
	PRs     []PRResult    `json:"prs,omitempty"`
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
	Client            *ai.Client
	UserRepo          *user.Repo
	NameHandler       *name.Handler
	SessionSvc        *session.Service
	SessionRepo       *session.Repo
	LogentrySvc       *logentry.Service
	LogentryRepo      *logentry.Repo
	ExerciseSvc       *exercise.Service
	ExerciseRepo      *exercise.Repo
	QuerySvc          *query.Service
	CorrectionSvc     *correction.Service
	PrSvc             *pr.Service
	PrRepo            *pr.Repo
	NotesRepo         *notes.Repo
	ChatMessagesRepo  *chatmessages.Repo
	R2                *storage.R2
	CommandExecutor   *command.Executor
}

type Service struct {
	client            *ai.Client
	userRepo          *user.Repo
	nameHandler       *name.Handler
	sessionSvc        *session.Service
	logentrySvc       *logentry.Service
	logentryRepo      *logentry.Repo
	sessionRepo       *session.Repo
	exerciseSvc       *exercise.Service
	exerciseRepo      *exercise.Repo
	querySvc          *query.Service
	correctionSvc     *correction.Service
	prSvc             *pr.Service
	prRepo            *pr.Repo
	notesRepo         *notes.Repo
	chatMessagesRepo  *chatmessages.Repo
	r2                *storage.R2
	workoutCtxBuilder  *workoutcontext.Builder
	commandExecutor   *command.Executor
}

func NewService(cfg Config) *Service {
	var wcBuilder *workoutcontext.Builder
	if cfg.SessionRepo != nil && cfg.LogentryRepo != nil && cfg.ExerciseRepo != nil {
		wcBuilder = workoutcontext.NewBuilder(cfg.SessionRepo, cfg.LogentryRepo, cfg.ExerciseRepo)
	}
	return &Service{
		client:            cfg.Client,
		userRepo:          cfg.UserRepo,
		nameHandler:       cfg.NameHandler,
		sessionSvc:        cfg.SessionSvc,
		logentrySvc:       cfg.LogentrySvc,
		logentryRepo:      cfg.LogentryRepo,
		sessionRepo:       cfg.SessionRepo,
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
		commandExecutor:   cfg.CommandExecutor,
	}
}

// Process handles text or audio and returns the response.
// Uses agent loop with tool calling: LLM may respond directly or call query_history/execute_commands.
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
		return &Response{Message: "Didn't catch that — try again?"}, nil
	}

	var wc *workoutcontext.WorkoutContext
	var workoutCtxStr string
	if s.workoutCtxBuilder != nil {
		var err error
		wc, err = s.workoutCtxBuilder.Build(ctx, userID)
		if err != nil {
			slog.Warn("workout context build failed", "err", err)
			if isInfrastructureError(err) {
				return &Response{Message: "Something went wrong. Try again."}, nil
			}
		} else {
			workoutCtxStr = wc.FormatForLLM()
		}
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	systemPrompt := ai.AgentSystemPrompt(today, yesterday, u.Name, workoutCtxStr)

	recent := s.loadRecentMessages(ctx, userID)
	openaiMsgs := make([]openai.ChatCompletionMessage, 0, 2+len(recent)+1)
	openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{Role: "system", Content: systemPrompt})
	for _, m := range recent {
		openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{Role: m.Role, Content: m.Content})
	}
	openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{Role: "user", Content: text})

	tools := ai.ChatTools()
	const maxIterations = 3
	var finalMessage string
	var finalEntries []LogResult
	var finalHistory *HistoryResult
	var finalPRs []PRResult

	for iter := 0; iter < maxIterations; iter++ {
		content, toolCalls, err := s.client.ChatWithToolsRaw(ctx, userID, openaiMsgs, tools)
		if err != nil {
			if isInfrastructureError(err) {
				return &Response{Message: "Something went wrong. Try again."}, nil
			}
			return nil, fmt.Errorf("chat: %w", err)
		}

		if len(toolCalls) == 0 {
			finalMessage = content
			break
		}

		openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		})

		for _, tc := range toolCalls {
			var result string
			if tc.Function.Name == "query_history" {
				hist, r, err := s.runQueryHistory(ctx, userID, tc.Function.Arguments)
				if err != nil {
					result = "error: " + err.Error()
				} else {
					result = r
					finalHistory = hist
				}
			} else if tc.Function.Name == "execute_commands" {
				execResult, entries, prs, err := s.runExecuteCommands(ctx, userID, wc, tc.Function.Arguments)
				if err != nil {
					result = "error: " + err.Error()
				} else if execResult.Success {
					finalEntries = entries
					finalPRs = prs
					if len(execResult.PRs) > 0 {
						prJSON, _ := json.Marshal(execResult.PRs)
						result = string(prJSON) + "\n\nFormat a celebratory message for these PRs."
					} else if args := parseExecuteArgs(tc.Function.Arguments); args != nil && args.SuccessMessage != "" {
						result = "success: " + args.SuccessMessage
					} else {
						result = "success"
					}
				} else {
					result = "error: " + execResult.Error
				}
			} else {
				result = "unknown tool"
			}
			openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	if finalMessage == "" && len(openaiMsgs) > 0 {
		content, _, err := s.client.ChatWithToolsRaw(ctx, userID, openaiMsgs, tools)
		if err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		finalMessage = content
	}

	resp := &Response{Message: finalMessage, Entries: finalEntries, History: finalHistory, PRs: finalPRs}
	s.appendMessages(ctx, userID, text, resp.Message)
	return resp, nil
}

func (s *Service) runQueryHistory(ctx context.Context, userID uuid.UUID, argsJSON string) (*HistoryResult, string, error) {
	var args struct {
		Category  string `json:"category"`
		Variant   string `json:"variant"`
		Scope     string `json:"scope"`
		Metric    string `json:"metric"`
		FromDate  string `json:"from_date"`
		ToDate    string `json:"to_date"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, "", err
	}
	if args.Category == "" {
		return nil, "", fmt.Errorf("category required")
	}
	if args.Variant == "" {
		args.Variant = "standard"
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}
	params := query.QueryParams{
		Category: args.Category,
		Variant:  args.Variant,
		Scope:    args.Scope,
		Metric:   args.Metric,
		FromDate: args.FromDate,
		ToDate:   args.ToDate,
		Limit:    args.Limit,
	}
	res, err := s.querySvc.Query(ctx, userID, params)
	if err != nil {
		return nil, "", err
	}
	emptyHist := &HistoryResult{ExerciseName: "", VariantName: "", Entries: []interface{}{}}
	if res == nil {
		return emptyHist, `{"exercise_name":"","variant_name":"","entries":[],"message":"Nothing logged for that yet."}`, nil
	}
	entries := make([]interface{}, len(res.Entries))
	for i, e := range res.Entries {
		sets := make([]map[string]interface{}, len(e.Sets))
		for j, set := range e.Sets {
			reps := set.Reps
			if reps == 0 && set.Weight != nil && *set.Weight > 0 {
				reps = 1
			}
			sets[j] = map[string]interface{}{"weight": set.Weight, "reps": reps, "set_type": set.SetType}
		}
		entries[i] = map[string]interface{}{
			"session_date": e.SessionDate,
			"raw_speech":   e.RawSpeech,
			"sets":         sets,
			"created_at":   e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	hist := &HistoryResult{
		ExerciseName: res.ExerciseName,
		VariantName:  res.VariantName,
		Entries:      entries,
	}
	out, _ := json.Marshal(res)
	return hist, string(out), nil
}

func (s *Service) runExecuteCommands(ctx context.Context, userID uuid.UUID, wc *workoutcontext.WorkoutContext, argsJSON string) (*command.ExecutionResult, []LogResult, []PRResult, error) {
	var args executeArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, nil, nil, err
	}
	if s.commandExecutor == nil {
		return &command.ExecutionResult{Success: false, Error: "commands not available"}, nil, nil, nil
	}
	defaultDate := time.Now().Format("2006-01-02")
	result := s.commandExecutor.Execute(ctx, userID, args.Commands, wc, defaultDate)

	var entries []LogResult
	var prs []PRResult
	if result.Success {
		for _, eid := range result.CreatedEntryIDs {
			entry, _ := s.logentryRepo.GetByID(ctx, uuid.MustParse(eid))
			if entry != nil {
				v, _ := s.exerciseRepo.GetVariantByID(ctx, entry.ExerciseVariantID)
				catName, varName := "", ""
				if v != nil {
					if cat, _ := s.exerciseRepo.GetCategoryByID(ctx, v.CategoryID); cat != nil {
						catName = cat.Name
					}
					varName = v.Name
				}
				sess, _ := s.sessionRepo.GetByID(ctx, entry.SessionID)
				date := ""
				if sess != nil {
					date = sess.Date.Format("2006-01-02")
				}
				entries = append(entries, LogResult{ExerciseName: catName, VariantName: varName, SessionDate: date, EntryID: eid})
			}
		}
		for _, p := range result.PRs {
			prs = append(prs, PRResult{ID: p.ID, Exercise: p.Exercise, Variant: p.Variant, Weight: p.Weight, Reps: p.Reps, PRType: p.PRType})
			if s.r2 != nil {
				prRec := &pr.PersonalRecord{ID: uuid.MustParse(p.ID), UserID: userID, Weight: p.Weight, Reps: p.Reps, PRType: p.PRType}
				s.generateAndUploadPRImage(ctx, userID, prRec, p.Exercise)
			}
		}
	}
	return result, entries, prs, nil
}

func isInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "no such host")
}

type executeArgs struct {
	Commands        []command.Command `json:"commands"`
	SuccessMessage  string           `json:"success_message"`
}

func parseExecuteArgs(args string) *executeArgs {
	var a executeArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return nil
	}
	return &a
}

const contextWindowSize = 6

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

func (s *Service) appendMessages(ctx context.Context, userID uuid.UUID, userText string, assistantMessage string) {
	if s.chatMessagesRepo == nil {
		return
	}
	_ = s.chatMessagesRepo.Append(ctx, userID, "user", userText)
	_ = s.chatMessagesRepo.Append(ctx, userID, "assistant", assistantMessage)
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
