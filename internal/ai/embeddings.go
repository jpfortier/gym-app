package ai

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

// Embed generates an embedding for text. Returns nil in test mode.
func (c *Client) Embed(ctx context.Context, userID uuid.UUID, text string) ([]float32, error) {
	if c.testMode {
		return nil, nil
	}
	if c.client == nil {
		return nil, fmt.Errorf("openai client not configured")
	}
	if err := c.throttle.Allow(ctx, userID); err != nil {
		return nil, err
	}
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{text},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	if c.usage != nil {
		prompt, completion := resp.Usage.PromptTokens, resp.Usage.CompletionTokens
		cost := CostCents("text-embedding-3-small", prompt, completion)
		c.usage.Record(ctx, &userID, "text-embedding-3-small", prompt, completion, cost)
	}
	return resp.Data[0].Embedding, nil
}
