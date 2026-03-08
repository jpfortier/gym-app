package logentry

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/db"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, entry *LogEntry, sets []SetInput) error {
	db.EnsureV7(&entry.ID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO log_entries (id, session_id, exercise_variant_id, raw_speech, notes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		entry.ID, entry.SessionID, entry.ExerciseVariantID, db.NullStr(entry.RawSpeech), db.NullStr(entry.Notes),
	).Scan(&entry.CreatedAt)
	if err != nil {
		return err
	}

	for i := range sets {
		setID := uuid.Must(uuid.NewV7())
		_, err = tx.ExecContext(ctx,
			`INSERT INTO log_entry_sets (id, log_entry_id, weight, reps, set_order, set_type)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			setID, entry.ID, db.NullFloat64(sets[i].Weight), sets[i].Reps, sets[i].SetOrder, db.NullStr(sets[i].SetType),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*LogEntry, error) {
	var e LogEntry
	err := r.db.QueryRowContext(ctx,
		`SELECT id, session_id, exercise_variant_id, COALESCE(raw_speech,''), COALESCE(notes,''), disabled_at, created_at
		 FROM log_entries WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.SessionID, &e.ExerciseVariantID, &e.RawSpeech, &e.Notes, &e.DisabledAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sets, err := r.setsForEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Sets = sets
	return &e, nil
}

func (r *Repo) ListByUserAndVariant(ctx context.Context, userID, variantID uuid.UUID, limit int) ([]*LogEntry, error) {
	return r.ListByUserAndVariantWithDateRange(ctx, userID, variantID, "", "", limit)
}

func (r *Repo) ListByUserAndVariantWithDateRange(ctx context.Context, userID, variantID uuid.UUID, fromDate, toDate string, limit int) ([]*LogEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	var fromVal, toVal interface{}
	if fromDate != "" {
		fromVal = fromDate
	} else {
		fromVal = "1900-01-01"
	}
	if toDate != "" {
		toVal = toDate
	} else {
		toVal = "2100-01-01"
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.session_id, e.exercise_variant_id, COALESCE(e.raw_speech,''), COALESCE(e.notes,''), e.disabled_at, e.created_at
		 FROM log_entries e
		 JOIN workout_sessions s ON e.session_id = s.id
		 WHERE s.user_id = $1 AND e.exercise_variant_id = $2 AND e.disabled_at IS NULL
		 AND s.date >= $3::date AND s.date <= $4::date
		 ORDER BY e.created_at DESC LIMIT $5`,
		userID, variantID, fromVal, toVal, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.ExerciseVariantID, &e.RawSpeech, &e.Notes, &e.DisabledAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		sets, err := r.setsForEntry(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		e.Sets = sets
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (r *Repo) GetMostRecentEntryForUser(ctx context.Context, userID uuid.UUID, date string) (*LogEntry, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	var e LogEntry
	err := r.db.QueryRowContext(ctx,
		`SELECT e.id, e.session_id, e.exercise_variant_id, COALESCE(e.raw_speech,''), COALESCE(e.notes,''), e.disabled_at, e.created_at
		 FROM log_entries e
		 JOIN workout_sessions s ON e.session_id = s.id
		 WHERE s.user_id = $1 AND e.disabled_at IS NULL AND s.date = $2::date
		 ORDER BY e.created_at DESC LIMIT 1`,
		userID, date,
	).Scan(&e.ID, &e.SessionID, &e.ExerciseVariantID, &e.RawSpeech, &e.Notes, &e.DisabledAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sets, err := r.setsForEntry(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	e.Sets = sets
	return &e, nil
}

func (r *Repo) ListBySession(ctx context.Context, sessionID uuid.UUID) ([]*LogEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, session_id, exercise_variant_id, COALESCE(raw_speech,''), COALESCE(notes,''), disabled_at, created_at
		 FROM log_entries WHERE session_id = $1 AND disabled_at IS NULL ORDER BY created_at`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.ExerciseVariantID, &e.RawSpeech, &e.Notes, &e.DisabledAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		sets, err := r.setsForEntry(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		e.Sets = sets
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (r *Repo) setsForEntry(ctx context.Context, entryID uuid.UUID) ([]LogEntrySet, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, log_entry_id, weight, reps, set_order, COALESCE(set_type,''), created_at
		 FROM log_entry_sets WHERE log_entry_id = $1 ORDER BY set_order`,
		entryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sets []LogEntrySet
	for rows.Next() {
		var s LogEntrySet
		var w sql.NullFloat64
		if err := rows.Scan(&s.ID, &s.LogEntryID, &w, &s.Reps, &s.SetOrder, &s.SetType, &s.CreatedAt); err != nil {
			return nil, err
		}
		if w.Valid {
			s.Weight = &w.Float64
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

func (r *Repo) DisableEntry(ctx context.Context, entryID uuid.UUID, userID uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE log_entries SET disabled_at = now()
		 WHERE id = $1 AND session_id IN (SELECT id FROM workout_sessions WHERE user_id = $2)
		 AND disabled_at IS NULL`,
		entryID, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *Repo) UpdateSet(ctx context.Context, setID uuid.UUID, weight *float64, reps *int) error {
	if weight != nil && reps != nil {
		_, err := r.db.ExecContext(ctx,
			`UPDATE log_entry_sets SET weight = $1, reps = $2 WHERE id = $3`,
			db.NullFloat64(weight), *reps, setID,
		)
		return err
	}
	if weight != nil {
		_, err := r.db.ExecContext(ctx,
			`UPDATE log_entry_sets SET weight = $1 WHERE id = $2`,
			db.NullFloat64(weight), setID,
		)
		return err
	}
	if reps != nil {
		_, err := r.db.ExecContext(ctx,
			`UPDATE log_entry_sets SET reps = $1 WHERE id = $2`,
			*reps, setID,
		)
		return err
	}
	return nil
}

