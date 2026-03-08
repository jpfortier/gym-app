package pr

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/db"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, pr *PersonalRecord) error {
	db.EnsureV7(&pr.ID)
	return r.db.QueryRowContext(ctx,
		`INSERT INTO personal_records (id, user_id, exercise_variant_id, pr_type, weight, reps, log_entry_set_id, image_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING created_at`,
		pr.ID, pr.UserID, pr.ExerciseVariantID, pr.PRType, pr.Weight, db.NullInt(pr.Reps), db.NullUUID(pr.LogEntrySetID), db.NullStr(pr.ImageURL),
	).Scan(&pr.CreatedAt)
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*PersonalRecord, error) {
	var pr PersonalRecord
	var reps sql.NullInt64
	var logEntrySetID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, exercise_variant_id, pr_type, weight, reps, log_entry_set_id, COALESCE(image_url,''), created_at
		 FROM personal_records WHERE id = $1`,
		id,
	).Scan(&pr.ID, &pr.UserID, &pr.ExerciseVariantID, &pr.PRType, &pr.Weight, &reps, &logEntrySetID, &pr.ImageURL, &pr.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pr.Reps = db.NullInt64ToIntPtr(reps)
	pr.LogEntrySetID = db.NullStringToUUIDPtr(logEntrySetID)
	return &pr, nil
}

func (r *Repo) ListByUser(ctx context.Context, userID uuid.UUID) ([]*PersonalRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, exercise_variant_id, pr_type, weight, reps, log_entry_set_id, COALESCE(image_url,''), created_at
		 FROM personal_records WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PersonalRecord
	for rows.Next() {
		var pr PersonalRecord
		var reps sql.NullInt64
		var logEntrySetID sql.NullString
		if err := rows.Scan(&pr.ID, &pr.UserID, &pr.ExerciseVariantID, &pr.PRType, &pr.Weight, &reps, &logEntrySetID, &pr.ImageURL, &pr.CreatedAt); err != nil {
			return nil, err
		}
		pr.Reps = db.NullInt64ToIntPtr(reps)
		pr.LogEntrySetID = db.NullStringToUUIDPtr(logEntrySetID)
		out = append(out, &pr)
	}
	return out, rows.Err()
}
