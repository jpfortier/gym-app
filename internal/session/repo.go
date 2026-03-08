package session

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, s *Session) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.Must(uuid.NewV7())
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO workout_sessions (id, user_id, date)
		 VALUES ($1, $2, $3::date)
		 RETURNING id, created_at`,
		s.ID, s.UserID, s.Date,
	).Scan(&s.ID, &s.CreatedAt)
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, date, created_at FROM workout_sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.UserID, &s.Date, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repo) GetByUserAndDate(ctx context.Context, userID uuid.UUID, date string) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, date, created_at FROM workout_sessions WHERE user_id = $1 AND date = $2::date`,
		userID, date,
	).Scan(&s.ID, &s.UserID, &s.Date, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*Session, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, date, created_at FROM workout_sessions
		 WHERE user_id = $1 ORDER BY date DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.Date, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}
