package ai

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

// GeneratePRImage creates a DALL-E image for a PR and returns the PNG bytes.
func (c *Client) GeneratePRImage(ctx context.Context, userID uuid.UUID, exerciseName string, weight float64, reps *int, prType string) ([]byte, error) {
	if c.testMode {
		return nil, nil
	}
	if c.client == nil {
		return nil, fmt.Errorf("openai client not configured")
	}
	if err := c.throttle.AllowDalle(ctx, userID); err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf("Cartoon illustration, anthropomorphic yellow and orange train locomotive with muscular arms performing %s. ", exerciseName)
	if weight > 0 {
		prompt += fmt.Sprintf("The weights are red train freight cars with \"%.0f\" in large yellow numbers. ", weight)
	} else if reps != nil {
		prompt += fmt.Sprintf("A sign showing \"%d\" reps in large numbers. ", *reps)
	}
	prompt += "Industrial warehouse gym setting, concrete floor, gritty texture. Bold comic book style. Celebrating a new personal record."
	req := openai.ImageRequest{
		Model:          openai.CreateImageModelDallE3,
		Prompt:         prompt,
		Size:           openai.CreateImageSize1024x1024,
		ResponseFormat: openai.CreateImageResponseFormatB64JSON,
		N:              1,
	}
	resp, err := c.client.CreateImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("dalle: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no image returned")
	}
	if c.usage != nil {
		cost := CostCents("dall-e-3", 0, 0)
		c.usage.Record(ctx, &userID, "dall-e-3", 0, 0, cost)
	}
	return base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
}
