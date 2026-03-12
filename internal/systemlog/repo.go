package systemlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jpfortier/gym-app/internal/db"
)

func fmtPlaceholder(n int) string { return fmt.Sprintf("%d", n) }

type Repo struct {
	db *sql.DB
}

func NewRepo(database *sql.DB) *Repo {
	return &Repo{db: database}
}

func (r *Repo) Insert(ctx context.Context, p InsertParams) error {
	var detailsJSON []byte
	if p.Details != nil {
		var err error
		detailsJSON, err = json.Marshal(p.Details)
		if err != nil {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO system_logs (category, user_id, method, path, details, error)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		p.Category, db.NullUUID(p.UserID), db.NullStr(p.Method), db.NullStr(p.Path),
		detailsJSON, db.NullStr(p.Error),
	)
	return err
}

func (r *Repo) List(ctx context.Context, params ListParams) ([]Entry, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	var args []interface{}
	query := `SELECT id, created_at, category, user_id, method, path, details, error
	          FROM system_logs WHERE 1=1`
	n := 1
	if params.Category != "" {
		query += ` AND category = $` + fmtPlaceholder(n)
		args = append(args, params.Category)
		n++
	}
	if params.UserID != nil {
		query += ` AND user_id = $` + fmtPlaceholder(n)
		args = append(args, *params.UserID)
		n++
	}
	query += ` ORDER BY created_at DESC LIMIT $` + fmtPlaceholder(n) + ` OFFSET $` + fmtPlaceholder(n+1)
	args = append(args, limit, params.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var userIDVal sql.NullString
		var methodVal, pathVal, errorVal sql.NullString
		var detailsJSON []byte
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Category, &userIDVal, &methodVal, &pathVal, &detailsJSON, &errorVal); err != nil {
			return nil, err
		}
		e.UserID = db.NullStringToUUIDPtr(userIDVal)
		e.Method = methodVal.String
		e.Path = pathVal.String
		e.Error = errorVal.String
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &e.Details)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
