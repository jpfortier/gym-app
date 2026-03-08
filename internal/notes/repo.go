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
