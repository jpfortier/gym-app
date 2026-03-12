package ai

import (
	"context"

	"github.com/google/uuid"
)

// UsageRecorder records AI API usage for billing/admin. Nil = no recording.
type UsageRecorder interface {
	Record(ctx context.Context, userID *uuid.UUID, model string, promptTokens, completionTokens int, costCents float64)
}

// CostCents returns estimated cost in cents for common models.
// Rates (approx): GPT-4o $2.50/1M in $10/1M out, embedding $0.02/1M, DALL-E 3 ~$0.04/img, Whisper $0.006/min
func CostCents(model string, promptTokens, completionTokens int) float64 {
	switch model {
	case "gpt-4o", "gpt-4o-2024-05-13":
		return float64(promptTokens)*0.00025 + float64(completionTokens)*0.001
	case "text-embedding-3-small":
		total := promptTokens + completionTokens
		return float64(total) * 0.00002
	case "dall-e-3":
		return 4.0
	case "gpt-image-1.5":
		return 4.0
	default:
		return 0
	}
}

// CostCentsWhisper returns cost in cents for Whisper (duration in seconds, $0.006/min).
func CostCentsWhisper(durationSeconds float64) float64 {
	return durationSeconds / 60 * 0.6
}
