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

// List returns ai_usage rows, optionally filtered by userID. limit defaults to 100.
func (r *Repo) List(ctx context.Context, userID *uuid.UUID, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if userID != nil {
		rows, err = r.db.QueryContext(ctx,
			`SELECT user_id, model, prompt_tokens, completion_tokens, estimated_cost_cents
			 FROM ai_usage WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
			*userID, limit,
		)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT user_id, model, prompt_tokens, completion_tokens, estimated_cost_cents
			 FROM ai_usage ORDER BY created_at DESC LIMIT $1`,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var rec Record
		var userIDVal sql.NullString
		if err := rows.Scan(&userIDVal, &rec.Model, &rec.PromptTokens, &rec.CompletionTokens, &rec.CostCents); err != nil {
			return nil, err
		}
		rec.UserID = db.NullStringToUUIDPtr(userIDVal)
		out = append(out, rec)
	}
	return out, rows.Err()
}
