package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"

	"github.com/jpfortier/gym-app/internal/env"
)

type Client struct {
	client   *openai.Client
	throttle *Throttler
	usage    UsageRecorder
	testMode bool
}

type ChatMessage struct {
	Role    string
	Content string
}

func NewClient(throttle *Throttler, usage UsageRecorder) *Client {
	key := env.OpenAIAPIKey()
	testMode := env.OpenAITestMode() || key == ""
	var client *openai.Client
	if !testMode {
		client = openai.NewClient(key)
	}
	return &Client{client: client, throttle: throttle, usage: usage, testMode: testMode}
}

func (c *Client) TestMode() bool {
	return c.testMode
}

// Transcribe decodes base64 audio and returns text. Throttled.
// fileExt is optional (e.g. "m4a", "webm"); defaults to "webm" if empty.
func (c *Client) Transcribe(ctx context.Context, userID uuid.UUID, audioBase64 string, fileExt string) (string, error) {
	if c.testMode {
		return "bench press 135 for 8", nil
	}
	if err := c.throttle.Allow(ctx, userID); err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	if fileExt == "" {
		fileExt = "webm"
	}
	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: "audio." + fileExt,
		Reader:   bytes.NewReader(data),
	}
	resp, err := c.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", fmt.Errorf("whisper: %w", err)
	}
	if c.usage != nil && resp.Duration > 0 {
		cost := CostCentsWhisper(resp.Duration)
		c.usage.Record(ctx, &userID, "whisper-1", 0, 0, cost)
	}
	return resp.Text, nil
}

// Chat sends messages and returns the assistant reply. Throttled.
func (c *Client) Chat(ctx context.Context, userID uuid.UUID, messages []ChatMessage) (string, error) {
	if c.testMode {
		return `{"intent":"log","date":"2025-03-20","exercises":[{"exercise_name":"Bench Press","variant_name":"standard","raw_speech":"bench 135x8","notes":"","sets":[{"weight":135,"reps":8,"set_type":"working","set_order":1}]}]}`, nil
	}
	if err := c.throttle.Allow(ctx, userID); err != nil {
		return "", err
	}
	msgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		msgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
	}
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    openai.GPT4o,
		Messages: msgs,
	})
	if err != nil {
		return "", fmt.Errorf("chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion")
	}
	if c.usage != nil {
		cost := CostCents("gpt-4o", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		c.usage.Record(ctx, &userID, "gpt-4o", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, cost)
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
