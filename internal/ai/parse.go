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
	Intent string `json:"intent"` // "log" | "query" | "correction" | "remove" | "restore" | "unknown"
	// Log
	Date      string          `json:"date,omitempty"`      // YYYY-MM-DD
	Exercises []ParsedExercise `json:"exercises,omitempty"`
	// Query + Correction + Remove
	Category  string            `json:"category,omitempty"`
	Variant   string            `json:"variant,omitempty"`
	TargetRef string            `json:"target_ref,omitempty"` // for correction: "last bench"; for remove: "last", "that"
	Changes   *ParsedCorrection `json:"changes,omitempty"`
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
- Log: {"intent":"log","date":"YYYY-MM-DD","exercises":[{"exercise_name":"Bench Press","variant_name":"standard","raw_speech":"bench 140x8","notes":"","sets":[{"weight":140,"reps":8,"set_type":"working","set_order":1}]}]}
  Use "standard" for default variant. Date: use today's date ({{.Today}}) unless user says yesterday, last Tuesday, etc.
- Query: {"intent":"query","category":"bench press","variant":"standard"}
- Correction: {"intent":"correction","target_ref":"last bench","changes":{"weight":150}}
- Remove: {"intent":"remove","category":"bench press","variant":"standard","target_ref":"last"}
  User wants to delete/remove a logged entry. Phrases: "forget that", "remove it", "delete the last bench", "scratch that".
  If user says "forget that" or "remove it" without naming exercise, omit category/variant (we remove most recent entry). Otherwise use category and variant. target_ref: "last" or "that".
- Restore: {"intent":"restore"}
  User wants to bring back a recently removed entry. Phrases: "bring that back", "oh sorry bring it back", "restore that", "undo that" (after having removed something).
- Unclear: {"intent":"unknown"}

Rules:
- exercise_name: use common names (Bench Press, Deadlift, Squat, etc). Lowercase for category in query.
- variant_name: "standard" unless user specifies (close grip, RDL, etc).
- sets: weight in lbs, reps as int. set_order 1,2,3... set_type optional (working, warm-up, etc).
- For bodyweight (push-ups, pull-ups): omit weight or null.
- date: always YYYY-MM-DD. Infer "today" as {{.Today}}, "yesterday" as {{.Yesterday}}.
- Output ONLY valid JSON, no markdown, no explanation.`

// Parser parses user text into structured intent.
type Parser interface {
	Parse(ctx context.Context, userID uuid.UUID, text string) (*ParsedIntent, error)
}

type parserImpl struct {
	client *Client
}

func NewParser(client *Client) Parser {
	return &parserImpl{client: client}
}

func (p *parserImpl) Parse(ctx context.Context, userID uuid.UUID, text string) (*ParsedIntent, error) {
	if p.client.testMode {
		return p.parseMock(text)
	}
	return p.parseReal(ctx, userID, text)
}

func (p *parserImpl) parseMock(text string) (*ParsedIntent, error) {
	text = strings.TrimSpace(strings.ToLower(text))
	if strings.Contains(text, "what") || strings.Contains(text, "how much") || strings.Contains(text, "history") {
		return &ParsedIntent{Intent: "query", Category: "bench press", Variant: "standard"}, nil
	}
	if strings.Contains(text, "change") || strings.Contains(text, "correct") || strings.Contains(text, "wrong") {
		return &ParsedIntent{Intent: "correction", TargetRef: "last", Changes: &ParsedCorrection{Weight: ptrFloat(150)}}, nil
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
		Date:   "2025-03-20",
		Exercises: []ParsedExercise{{
			ExerciseName: "Bench Press",
			VariantName:  "standard",
			RawSpeech:    text,
			Sets:         []ParsedSet{{Weight: &w, Reps: 8, SetType: "working", SetOrder: 1}},
		}},
	}, nil
}

func ptrFloat(f float64) *float64 { return &f }

func (p *parserImpl) parseReal(ctx context.Context, userID uuid.UUID, text string) (*ParsedIntent, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	prompt := strings.ReplaceAll(parsePrompt, "{{.Today}}", today)
	prompt = strings.ReplaceAll(prompt, "{{.Yesterday}}", yesterday)
	resp, err := p.client.Chat(ctx, userID, []ChatMessage{{Role: "system", Content: prompt}, {Role: "user", Content: text}})
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
