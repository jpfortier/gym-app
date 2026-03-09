package chatmessages

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Role      string
	Content   string
	CreatedAt time.Time
}

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// ListRecent returns the last N messages for the user, oldest first (for prompt ordering).
func (r *Repo) ListRecent(ctx context.Context, userID uuid.UUID, limit int) ([]Message, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, role, content, created_at
		 FROM chat_messages
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.UserID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// ListOlder returns up to limit messages older than beforeID, oldest first.
// Use for lazy loading when user scrolls up.
func (r *Repo) ListOlder(ctx context.Context, userID uuid.UUID, beforeID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, role, content, created_at
		 FROM chat_messages
		 WHERE user_id = $1 AND created_at < (SELECT created_at FROM chat_messages WHERE id = $2 AND user_id = $1)
		 ORDER BY created_at DESC
		 LIMIT $2`,
		userID, beforeID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.UserID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// Append inserts a message.
func (r *Repo) Append(ctx context.Context, userID uuid.UUID, role, content string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chat_messages (id, user_id, role, content)
		 VALUES (gen_random_uuid(), $1, $2, $3)`,
		userID, role, content,
	)
	return err
}
