package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ParsedIntent is the structured output from the LLM.
type ParsedIntent struct {
	Intent string `json:"intent"` // "log" | "query" | "correction" | "remove" | "restore" | "note" | "set_name" | "update_name" | "unknown"
	// Log
	Date      string          `json:"date,omitempty"`      // YYYY-MM-DD
	Exercises []ParsedExercise `json:"exercises,omitempty"`
	// Query + Correction + Remove
	Category  string            `json:"category,omitempty"`
	Variant   string            `json:"variant,omitempty"`
	TargetRef string            `json:"target_ref,omitempty"` // for correction: "last bench"; for remove: "last", "that"
	Changes   *ParsedCorrection `json:"changes,omitempty"`
	// Note
	NoteContent string `json:"note_content,omitempty"`
	// Set/Update name
	Name string `json:"name,omitempty"`
	// Assumption ledger: explicit vs inferred, ambiguities
	Assumptions []Assumption   `json:"assumptions,omitempty"`
	Ambiguities []string       `json:"ambiguities,omitempty"`
	UIText      *ParsedUIText  `json:"ui_text,omitempty"`
}

type Assumption struct {
	Kind  string `json:"kind"`  // e.g. "target_inferred", "date_inferred", "unit_default"
	Value string `json:"value"`
}

type ParsedUIText struct {
	Preview      string `json:"preview,omitempty"`
	Confirmation string `json:"confirmation,omitempty"`
}

type ParsedExercise struct {
	ExerciseName string        `json:"exercise_name"`
	VariantName  string        `json:"variant_name"`
	RawSpeech    string        `json:"raw_speech"`
	Notes        string        `json:"notes"`
	Sets         []ParsedSet   `json:"sets"`
}

type ParsedSet struct {
	Weight   *float64 `json:"weight"`
	Reps     int      `json:"reps"`
	SetType  string   `json:"set_type"`
	SetOrder int      `json:"set_order"`
}

type ParsedCorrection struct {
	Weight *float64 `json:"weight,omitempty"`
	Reps   *int     `json:"reps,omitempty"`
}

const parsePrompt = `You are a workout logging assistant. Parse the user's message and output JSON only.

Output format (one of):
- Log: {"intent":"log","date":"YYYY-MM-DD","exercises":[...],"assumptions":[],"ambiguities":[],"ui_text":{"preview":"..."}}
- Query: {"intent":"query","category":"bench press","variant":"standard","assumptions":[],"ambiguities":[]}
- Correction: {"intent":"correction","target_ref":"last bench","changes":{"weight":150},"assumptions":[],"ambiguities":[],"ui_text":{"preview":"Update your last bench set to 150 lb."}}
- Remove: {"intent":"remove","category":"bench press","variant":"standard","target_ref":"last","assumptions":[],"ambiguities":[],"ui_text":{"preview":"Remove the last bench entry."}}
  User wants to delete/remove a logged entry. If user says "forget that" or "remove it" without naming exercise, omit category/variant.
- Restore: {"intent":"restore","assumptions":[],"ambiguities":[]}
- Note: {"intent":"note","category":"deadlift","variant":"rdl","note_content":"warm up hamstrings first","assumptions":[],"ambiguities":[]}
- Set name: {"intent":"set_name","name":"Peter"} — ONLY when user_name is empty. Accept: "my name is X", "I'm X", "call me X", or a single word X.
- Update name: {"intent":"update_name","name":"Mary"} — ONLY when user_name is set AND user explicitly asks to change: "change my name to X", "update my name to X", "my name is actually X". Do NOT use for casual "Peter" or "Mary" when name is set.
- Unclear: {"intent":"unknown","ambiguities":["unclear_intent"]}

Required fields for ALL intents:
- assumptions: array of {kind, value} for inferred facts. Examples: {"kind":"target_inferred","value":"last_created_set"}, {"kind":"date_inferred","value":"today"}, {"kind":"unit_default","value":"lb"}
- ambiguities: array of strings. List ANY uncertainty: "target_unclear", "multiple_targets", "exercise_ambiguous", "date_ambiguous", "unclear_intent". If confident, use [].

For correction/remove: include ui_text.preview with a short human-readable description of the action.

Rules:
- exercise_name: use common names (Bench Press, Deadlift, Squat, etc). Lowercase for category in query.
- variant_name: "standard" unless user specifies.
- sets: weight in lbs, reps as int. set_order 1,2,3... set_type optional.
- For bodyweight: omit weight or null.
- date: always YYYY-MM-DD. Infer "today" as {{.Today}}, "yesterday" as {{.Yesterday}}.
- Output ONLY valid JSON, no markdown, no explanation.

Context: You may receive WORKOUT_CONTEXT and recent conversation.
User context: user_name is "{{.UserName}}" (empty means name not set). Use for set_name vs update_name rules above. Use them to resolve "that", "another one", "last set", "change it" — they refer to the most recent relevant action. If target could match multiple entries (e.g. squat in today AND yesterday), add "multiple_targets" to ambiguities.`

// Parser parses user text into structured intent.
type Parser interface {
	Parse(ctx context.Context, userID uuid.UUID, text string, recentMessages []ChatMessage, workoutContext string, userName string) (*ParsedIntent, error)
}

type parserImpl struct {
	client *Client
}

func NewParser(client *Client) Parser {
	return &parserImpl{client: client}
}

func (p *parserImpl) Parse(ctx context.Context, userID uuid.UUID, text string, recentMessages []ChatMessage, workoutContext string, userName string) (*ParsedIntent, error) {
	if p.client.testMode {
		return p.parseMock(text, userName)
	}
	return p.parseReal(ctx, userID, text, recentMessages, workoutContext, userName)
}

var exerciseWords = map[string]bool{
	"bench": true, "squat": true, "deadlift": true, "press": true, "row": true, "curl": true,
	"rdl": true, "ohp": true, "pull": true, "push": true, "lunge": true, "plank": true,
}

func (p *parserImpl) parseMock(text string, userName string) (*ParsedIntent, error) {
	text = strings.TrimSpace(strings.ToLower(text))
	if userName == "" {
		if strings.HasPrefix(text, "my name is ") || strings.HasPrefix(text, "i'm ") || strings.HasPrefix(text, "call me ") {
			var name string
			switch {
			case strings.HasPrefix(text, "my name is "):
				name = strings.TrimPrefix(text, "my name is ")
			case strings.HasPrefix(text, "i'm "):
				name = strings.TrimPrefix(text, "i'm ")
			default:
				name = strings.TrimPrefix(text, "call me ")
			}
			return &ParsedIntent{Intent: "set_name", Name: strings.TrimSpace(name)}, nil
		}
		if !strings.Contains(text, " ") && !exerciseWords[text] && len(text) > 1 {
			return &ParsedIntent{Intent: "set_name", Name: text}, nil
		}
	}
	if userName != "" && (strings.Contains(text, "change") && strings.Contains(text, "name") || strings.Contains(text, "update") && strings.Contains(text, "name") || strings.HasPrefix(text, "my name is ")) {
		name := text
		if strings.HasPrefix(text, "my name is ") {
			name = strings.TrimPrefix(text, "my name is ")
		} else {
			parts := strings.Fields(text)
			for i, part := range parts {
				if (part == "to" || part == "is") && i+1 < len(parts) {
					name = strings.Join(parts[i+1:], " ")
					break
				}
			}
		}
		return &ParsedIntent{Intent: "update_name", Name: strings.TrimSpace(name)}, nil
	}
	if strings.Contains(text, "what") || strings.Contains(text, "how much") || strings.Contains(text, "history") {
		return &ParsedIntent{Intent: "query", Category: "bench press", Variant: "standard"}, nil
	}
	if strings.Contains(text, "change") || strings.Contains(text, "correct") || strings.Contains(text, "wrong") {
		return &ParsedIntent{Intent: "correction", TargetRef: "last", Changes: &ParsedCorrection{Weight: ptrFloat(150)}}, nil
	}
	if strings.Contains(text, "remember") || (strings.Contains(text, "note") && strings.Contains(text, "for")) {
		return &ParsedIntent{Intent: "note", Category: "deadlift", Variant: "rdl", NoteContent: "warm up hamstrings first"}, nil
	}
	if (strings.Contains(text, "bring") && strings.Contains(text, "back")) || strings.Contains(text, "restore") ||
		(strings.Contains(text, "sorry") && strings.Contains(text, "back")) {
		return &ParsedIntent{Intent: "restore"}, nil
	}
	if strings.Contains(text, "forget") || strings.Contains(text, "remove") || strings.Contains(text, "delete") ||
		strings.Contains(text, "undo") || strings.Contains(text, "scratch") {
		if strings.Contains(text, "bench") {
			return &ParsedIntent{Intent: "remove", Category: "bench press", Variant: "standard", TargetRef: "last"}, nil
		}
		return &ParsedIntent{Intent: "remove", TargetRef: "that"}, nil
	}
	w := 135.0
	return &ParsedIntent{
		Intent: "log",
		Date:   time.Now().Format("2006-01-02"),
		Exercises: []ParsedExercise{{
			ExerciseName: "Bench Press",
			VariantName:  "standard",
			RawSpeech:    text,
			Sets:         []ParsedSet{{Weight: &w, Reps: 8, SetType: "working", SetOrder: 1}},
		}},
	}, nil
}

func ptrFloat(f float64) *float64 { return &f }

func (p *parserImpl) parseReal(ctx context.Context, userID uuid.UUID, text string, recentMessages []ChatMessage, workoutContext string, userName string) (*ParsedIntent, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	prompt := strings.ReplaceAll(parsePrompt, "{{.Today}}", today)
	prompt = strings.ReplaceAll(prompt, "{{.Yesterday}}", yesterday)
	prompt = strings.ReplaceAll(prompt, "{{.UserName}}", userName)
	if workoutContext != "" {
		prompt += "\n\nWORKOUT_CONTEXT (use for resolving \"that\", \"last set\", \"another one\", etc.):\n" + workoutContext
	}
	msgs := make([]ChatMessage, 0, 2+len(recentMessages)+1)
	msgs = append(msgs, ChatMessage{Role: "system", Content: prompt})
	msgs = append(msgs, recentMessages...)
	msgs = append(msgs, ChatMessage{Role: "user", Content: text})
	resp, err := p.client.Chat(ctx, userID, msgs)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	// Strip markdown code blocks and extract JSON (LLM sometimes wraps in ```json ... ``` or adds trailing text)
	body := strings.TrimSpace(resp)
	body = strings.TrimPrefix(body, "```json")
	body = strings.TrimPrefix(body, "```")
	body = strings.TrimSpace(body)
	if idx := strings.Index(body, "```"); idx >= 0 {
		body = body[:idx]
	}
	if start := strings.Index(body, "{"); start >= 0 {
		if end := strings.LastIndex(body, "}"); end >= start {
			body = body[start : end+1]
		}
	}
	var out ParsedIntent
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	if out.Intent == "" {
		out.Intent = "unknown"
	}
	return &out, nil
}
