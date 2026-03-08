package exercise

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateCategory(ctx context.Context, c *Category) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.Must(uuid.NewV7())
	}
	var userID interface{}
	if c.UserID != nil {
		userID = *c.UserID
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO exercise_categories (id, user_id, name, show_weight, show_reps)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		c.ID, userID, c.Name, c.ShowWeight, c.ShowReps,
	).Scan(&c.CreatedAt)
}

func (r *Repo) GetCategoryByID(ctx context.Context, id uuid.UUID) (*Category, error) {
	var c Category
	var userID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, show_weight, show_reps, created_at FROM exercise_categories WHERE id = $1`,
		id,
	).Scan(&c.ID, &userID, &c.Name, &c.ShowWeight, &c.ShowReps, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		u, _ := uuid.Parse(userID.String)
		c.UserID = &u
	}
	return &c, nil
}

func (r *Repo) GetCategoryByUserAndName(ctx context.Context, userID *uuid.UUID, name string) (*Category, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	var userVal interface{}
	if userID != nil {
		userVal = *userID
	}
	var c Category
	var uid sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, show_weight, show_reps, created_at FROM exercise_categories
		 WHERE (user_id IS NOT DISTINCT FROM $1) AND LOWER(name) = $2`,
		userVal, name,
	).Scan(&c.ID, &uid, &c.Name, &c.ShowWeight, &c.ShowReps, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if uid.Valid {
		u, _ := uuid.Parse(uid.String)
		c.UserID = &u
	}
	return &c, nil
}

func (r *Repo) CreateVariant(ctx context.Context, v *Variant) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.Must(uuid.NewV7())
	}
	var userID interface{}
	if v.UserID != nil {
		userID = *v.UserID
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO exercise_variants (id, category_id, user_id, name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING created_at`,
		v.ID, v.CategoryID, userID, v.Name,
	).Scan(&v.CreatedAt)
}

func (r *Repo) GetVariantByID(ctx context.Context, id uuid.UUID) (*Variant, error) {
	var v Variant
	var userID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, category_id, user_id, name, created_at FROM exercise_variants WHERE id = $1`,
		id,
	).Scan(&v.ID, &v.CategoryID, &userID, &v.Name, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		u, _ := uuid.Parse(userID.String)
		v.UserID = &u
	}
	return &v, nil
}

func (r *Repo) GetVariantByCategoryAndName(ctx context.Context, categoryID uuid.UUID, userID *uuid.UUID, name string) (*Variant, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	var userVal interface{}
	if userID != nil {
		userVal = *userID
	}
	var v Variant
	var uid sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, category_id, user_id, name, created_at FROM exercise_variants
		 WHERE category_id = $1 AND (user_id IS NOT DISTINCT FROM $2) AND LOWER(name) = $3`,
		categoryID, userVal, name,
	).Scan(&v.ID, &v.CategoryID, &uid, &v.Name, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if uid.Valid {
		u, _ := uuid.Parse(uid.String)
		v.UserID = &u
	}
	return &v, nil
}

func (r *Repo) ListCategoriesForUser(ctx context.Context, userID uuid.UUID) ([]*Category, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, show_weight, show_reps, created_at FROM exercise_categories
		 WHERE user_id IS NULL OR user_id = $1 ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func scanCategories(rows *sql.Rows) ([]*Category, error) {
	var out []*Category
	for rows.Next() {
		var c Category
		var uid sql.NullString
		if err := rows.Scan(&c.ID, &uid, &c.Name, &c.ShowWeight, &c.ShowReps, &c.CreatedAt); err != nil {
			return nil, err
		}
		if uid.Valid {
			u, _ := uuid.Parse(uid.String)
			c.UserID = &u
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}
