package usage

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/db"
)

type Record struct {
	UserID           *uuid.UUID
	Model            string
	PromptTokens     int
	CompletionTokens int
	CostCents        float64
}

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) insert(ctx context.Context, rec Record) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ai_usage (user_id, model, prompt_tokens, completion_tokens, estimated_cost_cents)
		 VALUES ($1, $2, $3, $4, $5)`,
		db.NullUUID(rec.UserID), rec.Model, rec.PromptTokens, rec.CompletionTokens, rec.CostCents,
	)
	return err
}

// Record implements ai.UsageRecorder for persistence.
func (r *Repo) Record(ctx context.Context, userID *uuid.UUID, model string, promptTokens, completionTokens int, costCents float64) {
	_ = r.insert(ctx, Record{
		UserID:           userID,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		CostCents:        costCents,
	})
}
