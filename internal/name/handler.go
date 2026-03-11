package name

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/ai"
)

// Handler processes name-setting: AI twist for initial set, rude handling, and gym-bro response.
type Handler struct {
	client *ai.Client
}

func NewHandler(client *ai.Client) *Handler {
	return &Handler{client: client}
}

// Process returns the name to store and the response message.
// For set_name (initial): may twist the name or substitute if rude.
// For update_name (rename): uses rawName as-is.
func (h *Handler) Process(ctx context.Context, userID uuid.UUID, rawName string, isRename bool) (storedName string, responseMessage string, err error) {
	rawName = strings.TrimRight(strings.TrimSpace(rawName), ".,!?")
	if rawName == "" {
		return "", "What should I call you?", nil
	}
	if isRename {
		return rawName, fmt.Sprintf("%s it is. Let's go.", rawName), nil
	}
	displayName, err := h.twistOrSubstitute(ctx, userID, rawName)
	if err != nil {
		return rawName, fmt.Sprintf("Nice to meet you, %s! Let's get started.", rawName), nil
	}
	return displayName, h.buildWelcomeMessage(displayName), nil
}

func (h *Handler) twistOrSubstitute(ctx context.Context, userID uuid.UUID, rawName string) (string, error) {
	if h.client.TestMode() {
		return mockTwist(rawName), nil
	}
	prompt := `Given the name the user gave: "` + rawName + `"
If it's rude, offensive, or inappropriate, return a sweet, frilly alternative like Daisy or Petunia. One word only.
Otherwise return a playful twist - like Brandon→Brando, Tony→Antonio, Joe→Giuseppe. Clearly intentional, not rude. One word.
Output ONLY the single word, nothing else.`
	msgs := []ai.ChatMessage{{Role: "user", Content: prompt}}
	resp, err := h.client.Chat(ctx, userID, msgs)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(strings.Trim(resp, `"'`))
	if out == "" {
		return rawName, nil
	}
	return out, nil
}

func mockTwist(raw string) string {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "fuck") || strings.Contains(lower, "dumb") || len(raw) < 2 {
		return "Daisy"
	}
	switch lower {
	case "brandon":
		return "Brando"
	case "tony":
		return "Antonio"
	case "joe":
		return "Giuseppe"
	default:
		return raw
	}
}

func (h *Handler) buildWelcomeMessage(displayName string) string {
	return fmt.Sprintf("Alright %s, let's get started. Tell me what you crushed today — or ask me about your history once you've logged some. Time to punch your ticket to the gain train.", displayName)
}
