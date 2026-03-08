package pr

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

func (r *Repo) Create(ctx context.Context, pr *PersonalRecord) error {
	if pr.ID == uuid.Nil {
		pr.ID = uuid.Must(uuid.NewV7())
	}
	var reps, logEntrySetID interface{}
	if pr.Reps != nil {
		reps = *pr.Reps
	}
	if pr.LogEntrySetID != nil {
		logEntrySetID = *pr.LogEntrySetID
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO personal_records (id, user_id, exercise_variant_id, pr_type, weight, reps, log_entry_set_id, image_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING created_at`,
		pr.ID, pr.UserID, pr.ExerciseVariantID, pr.PRType, pr.Weight, reps, logEntrySetID, nullStr(pr.ImageURL),
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
	if reps.Valid {
		n := int(reps.Int64)
		pr.Reps = &n
	}
	if logEntrySetID.Valid {
		u, _ := uuid.Parse(logEntrySetID.String)
		pr.LogEntrySetID = &u
	}
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
		if reps.Valid {
			n := int(reps.Int64)
			pr.Reps = &n
		}
		if logEntrySetID.Valid {
			u, _ := uuid.Parse(logEntrySetID.String)
			pr.LogEntrySetID = &u
		}
		out = append(out, &pr)
	}
	return out, rows.Err()
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
