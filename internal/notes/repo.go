package notes

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/db"
)

type Note struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	ExerciseCategoryID *uuid.UUID
	ExerciseVariantID  *uuid.UUID
	Content            string
	CreatedAt          interface{}
}

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, userID uuid.UUID, categoryID, variantID *uuid.UUID, content string) (*Note, error) {
	n := &Note{UserID: userID, ExerciseCategoryID: categoryID, ExerciseVariantID: variantID, Content: content}
	db.EnsureV7(&n.ID)
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO notes (id, user_id, exercise_category_id, exercise_variant_id, content)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		n.ID, n.UserID, db.NullUUID(categoryID), db.NullUUID(variantID), content,
	).Scan(&n.CreatedAt)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ListByUser returns notes for a user, ordered by created_at DESC. limit defaults to 100.
func (r *Repo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*Note, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, exercise_category_id, exercise_variant_id, content, created_at
		 FROM notes WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Note
	for rows.Next() {
		var n Note
		var catID, varID sql.NullString
		if err := rows.Scan(&n.ID, &n.UserID, &catID, &varID, &n.Content, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.ExerciseCategoryID = db.NullStringToUUIDPtr(catID)
		n.ExerciseVariantID = db.NullStringToUUIDPtr(varID)
		out = append(out, &n)
	}
	return out, rows.Err()
}
