package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

type Client struct {
	client   *openai.Client
	throttle *Throttler
	testMode bool
}

type ChatMessage struct {
	Role    string
	Content string
}

func NewClient(throttle *Throttler) *Client {
	key := os.Getenv("OPENAI_API_KEY")
	testMode := strings.ToLower(os.Getenv("OPENAI_TEST_MODE")) == "true" || key == ""
	var client *openai.Client
	if !testMode {
		client = openai.NewClient(key)
	}
	return &Client{client: client, throttle: throttle, testMode: testMode}
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
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
