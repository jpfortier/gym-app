package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
	Role       string
	Content    string
	ToolCallID string // For role=tool, the ID from the assistant's tool_call
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
		Format:   openai.AudioResponseFormatVerboseJSON,
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
	content, _, err := c.ChatWithTools(ctx, userID, messages, nil)
	return content, err
}

// ChatWithToolsRaw sends raw OpenAI messages. Used by the agent loop when messages include tool calls.
func (c *Client) ChatWithToolsRaw(ctx context.Context, userID uuid.UUID, messages []openai.ChatCompletionMessage, tools []openai.Tool) (content string, toolCalls []openai.ToolCall, err error) {
	if c.testMode {
		chatMsgs := make([]ChatMessage, len(messages))
		for i, m := range messages {
			chatMsgs[i] = ChatMessage{Role: m.Role, Content: m.Content}
		}
		return c.chatWithToolsMock(chatMsgs, tools)
	}
	if err := c.throttle.Allow(ctx, userID); err != nil {
		return "", nil, err
	}
	req := openai.ChatCompletionRequest{Model: openai.GPT4o, Messages: messages}
	if len(tools) > 0 {
		req.Tools = tools
		req.ToolChoice = "auto"
	}
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", nil, fmt.Errorf("chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("no completion")
	}
	msg := resp.Choices[0].Message
	if c.usage != nil {
		cost := CostCents("gpt-4o", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		c.usage.Record(ctx, &userID, "gpt-4o", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, cost)
	}
	return strings.TrimSpace(msg.Content), msg.ToolCalls, nil
}

// ChatWithTools sends messages with optional tools. Returns content, tool calls (if any), and error.
// When tools are provided and the model returns tool_calls, the caller should execute them,
// append ToolCallResult messages, and call again.
func (c *Client) ChatWithTools(ctx context.Context, userID uuid.UUID, messages []ChatMessage, tools []openai.Tool) (content string, toolCalls []openai.ToolCall, err error) {
	msgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		msgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
	}
	return c.ChatWithToolsRaw(ctx, userID, msgs, tools)
}

func (c *Client) chatWithToolsMock(messages []ChatMessage, tools []openai.Tool) (string, []openai.ToolCall, error) {
	lastRole := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" {
			lastRole = "tool"
			break
		}
		if messages[i].Role == "user" {
			lastRole = "user"
			break
		}
	}
	if lastRole == "tool" && len(tools) > 0 {
		lastToolContent := ""
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "tool" {
				lastToolContent = messages[i].Content
				break
			}
		}
		if strings.HasPrefix(lastToolContent, "success: ") {
			return strings.TrimPrefix(lastToolContent, "success: "), nil, nil
		}
		return "Logged bench press **135×8** for today.", nil, nil
	}
	if len(tools) > 0 {
		lastContent := ""
		for _, m := range messages {
			if m.Role == "user" {
				lastContent = m.Content
			}
		}
		if strings.Contains(strings.ToLower(lastContent), "what") || strings.Contains(strings.ToLower(lastContent), "how much") {
			return "", []openai.ToolCall{{
				ID:   "call_mock_query",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "query_history",
					Arguments: `{"category":"bench press","variant":"standard","scope":"most_recent"}`,
				},
			}}, nil
		}
		lower := strings.ToLower(lastContent)
		if strings.HasPrefix(lower, "my name is ") || strings.HasPrefix(lower, "call me ") || strings.HasPrefix(lower, "i'm ") {
			name := strings.TrimSpace(lastContent)
			for _, prefix := range []string{"my name is ", "call me ", "i'm "} {
				if strings.HasPrefix(strings.ToLower(name), prefix) {
					name = strings.TrimSpace(name[len(prefix):])
					break
				}
			}
			if name != "" {
				args, _ := json.Marshal(map[string]interface{}{
					"commands":        []map[string]string{{"type": "SET_NAME", "name": name}},
					"success_message": "Alright " + name + ", let's get started.",
				})
				return "", []openai.ToolCall{{
					ID:   "call_mock_name",
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      "execute_commands",
						Arguments: string(args),
					},
				}}, nil
			}
		}
		words := strings.Fields(lastContent)
		if len(words) == 1 && len(words[0]) > 1 && !strings.ContainsAny(words[0], "0123456789") && !map[string]bool{"bench": true, "squat": true, "deadlift": true}[strings.ToLower(words[0])] {
			n := words[0]
			args, _ := json.Marshal(map[string]interface{}{
				"commands":        []map[string]string{{"type": "SET_NAME", "name": n}},
				"success_message": "Alright " + n + ", let's get started.",
			})
			return "", []openai.ToolCall{{
				ID:   "call_mock_name",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "execute_commands",
					Arguments: string(args),
				},
			}}, nil
		}
		if strings.Contains(strings.ToLower(lastContent), "bench") && (strings.Contains(lastContent, "135") || strings.Contains(lastContent, "140")) {
			return "", []openai.ToolCall{{
				ID:   "call_mock_exec",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "execute_commands",
					Arguments: `{"commands":[{"type":"CREATE_EXERCISE_ENTRY","exercise":"bench press","variant":"standard","raw_speech":"bench 135x8","sets":[{"weight":135,"reps":8}]}],"success_message":"Logged bench press **135×8** for today."}`,
				},
			}}, nil
		}
		if strings.Contains(strings.ToLower(lastContent), "forget") || strings.Contains(strings.ToLower(lastContent), "remove") {
			return "", []openai.ToolCall{{
				ID:   "call_mock_remove",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "execute_commands",
					Arguments: `{"commands":[{"type":"DISABLE_ENTRY","exercise":"bench press","variant":"standard"}],"success_message":"Scratched."}`,
				},
			}}, nil
		}
		if (strings.Contains(strings.ToLower(lastContent), "change") || strings.Contains(strings.ToLower(lastContent), "correct")) && strings.Contains(strings.ToLower(lastContent), "squat") {
			return "", []openai.ToolCall{{
				ID:   "call_mock_correct",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "execute_commands",
					Arguments: `{"commands":[{"type":"UPDATE_SET","target_ref":"last_created_set","changes":{"weight":205,"reps":5}}],"success_message":"Fixed."}`,
				},
			}}, nil
		}
		if (strings.Contains(strings.ToLower(lastContent), "bring") && strings.Contains(strings.ToLower(lastContent), "back")) || strings.Contains(strings.ToLower(lastContent), "restore") {
			return "", []openai.ToolCall{{
				ID:   "call_mock_restore",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "execute_commands",
					Arguments: `{"commands":[{"type":"RESTORE_ENTRY"}],"success_message":"Back in."}`,
				},
			}}, nil
		}
	}
	return "Got it. What else?", nil, nil
}
