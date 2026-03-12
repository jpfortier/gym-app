package exercise

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	"github.com/jpfortier/gym-app/internal/db"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateCategory(ctx context.Context, c *Category) error {
	db.EnsureV7(&c.ID)
	return r.db.QueryRowContext(ctx,
		`INSERT INTO exercise_categories (id, user_id, name, show_weight, show_reps)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		c.ID, db.NullUUID(c.UserID), c.Name, c.ShowWeight, c.ShowReps,
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
	c.UserID = db.NullStringToUUIDPtr(userID)
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
	c.UserID = db.NullStringToUUIDPtr(uid)
	return &c, nil
}

func (r *Repo) CreateVariant(ctx context.Context, v *Variant) error {
	db.EnsureV7(&v.ID)
	return r.db.QueryRowContext(ctx,
		`INSERT INTO exercise_variants (id, category_id, user_id, name, standard, visual_cues)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at`,
		v.ID, v.CategoryID, db.NullUUID(v.UserID), v.Name, v.Standard, db.NullStr(v.VisualCues),
	).Scan(&v.CreatedAt)
}

func (r *Repo) GetVariantByID(ctx context.Context, id uuid.UUID) (*Variant, error) {
	var v Variant
	var userID, visualCues sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, category_id, user_id, name, standard, visual_cues, created_at FROM exercise_variants WHERE id = $1`,
		id,
	).Scan(&v.ID, &v.CategoryID, &userID, &v.Name, &v.Standard, &visualCues, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.UserID = db.NullStringToUUIDPtr(userID)
	v.VisualCues = db.NullStringToString(visualCues)
	return &v, nil
}

func (r *Repo) GetVariantByCategoryAndName(ctx context.Context, categoryID uuid.UUID, userID *uuid.UUID, name string) (*Variant, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	var userVal interface{}
	if userID != nil {
		userVal = *userID
	}
	var v Variant
	var uid, visualCues sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, category_id, user_id, name, standard, visual_cues, created_at FROM exercise_variants
		 WHERE category_id = $1 AND (user_id IS NOT DISTINCT FROM $2) AND LOWER(name) = $3`,
		categoryID, userVal, name,
	).Scan(&v.ID, &v.CategoryID, &uid, &v.Name, &v.Standard, &visualCues, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.UserID = db.NullStringToUUIDPtr(uid)
	v.VisualCues = db.NullStringToString(visualCues)
	return &v, nil
}

func (r *Repo) ListVariantsByCategory(ctx context.Context, categoryID uuid.UUID, userID uuid.UUID) ([]*Variant, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, category_id, user_id, name, standard, visual_cues, created_at FROM exercise_variants
		 WHERE category_id = $1 AND (user_id IS NULL OR user_id = $2) ORDER BY name`,
		categoryID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Variant
	for rows.Next() {
		var v Variant
		var uid, visualCues sql.NullString
		if err := rows.Scan(&v.ID, &v.CategoryID, &uid, &v.Name, &v.Standard, &visualCues, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.UserID = db.NullStringToUUIDPtr(uid)
		v.VisualCues = db.NullStringToString(visualCues)
		out = append(out, &v)
	}
	return out, rows.Err()
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

func (r *Repo) FindCategoryByEmbedding(ctx context.Context, userID uuid.UUID, emb []float32, maxDistance float32) (*Category, error) {
	if len(emb) == 0 {
		return nil, nil
	}
	vec := pgvector.NewVector(emb)
	var c Category
	var uid sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, show_weight, show_reps, created_at
		 FROM exercise_categories
		 WHERE (user_id IS NULL OR user_id = $1) AND embedding IS NOT NULL
		   AND (embedding <=> $2) < $3
		 ORDER BY embedding <=> $2 LIMIT 1`,
		userID, vec, maxDistance,
	).Scan(&c.ID, &uid, &c.Name, &c.ShowWeight, &c.ShowReps, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.UserID = db.NullStringToUUIDPtr(uid)
	return &c, nil
}

func (r *Repo) FindVariantByEmbedding(ctx context.Context, categoryID uuid.UUID, userID uuid.UUID, emb []float32, maxDistance float32) (*Variant, error) {
	if len(emb) == 0 {
		return nil, nil
	}
	vec := pgvector.NewVector(emb)
	var v Variant
	var uid, visualCues sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, category_id, user_id, name, standard, visual_cues, created_at
		 FROM exercise_variants
		 WHERE category_id = $1 AND (user_id IS NULL OR user_id = $2) AND embedding IS NOT NULL
		   AND (embedding <=> $3) < $4
		 ORDER BY embedding <=> $3 LIMIT 1`,
		categoryID, userID, vec, maxDistance,
	).Scan(&v.ID, &v.CategoryID, &uid, &v.Name, &v.Standard, &visualCues, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.UserID = db.NullStringToUUIDPtr(uid)
	v.VisualCues = db.NullStringToString(visualCues)
	return &v, nil
}

func (r *Repo) UpdateCategoryEmbedding(ctx context.Context, id uuid.UUID, emb []float32) error {
	if len(emb) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE exercise_categories SET embedding = $1 WHERE id = $2`,
		pgvector.NewVector(emb), id,
	)
	return err
}

func (r *Repo) UpdateVariantEmbedding(ctx context.Context, id uuid.UUID, emb []float32) error {
	if len(emb) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE exercise_variants SET embedding = $1 WHERE id = $2`,
		pgvector.NewVector(emb), id,
	)
	return err
}

func (r *Repo) FindVariantByAlias(ctx context.Context, userID uuid.UUID, aliasKey string) (*Variant, error) {
	aliasKey = strings.TrimSpace(strings.ToLower(aliasKey))
	if aliasKey == "" {
		return nil, nil
	}
	var variantID uuid.UUID
	err := r.db.QueryRowContext(ctx,
		`SELECT variant_id FROM exercise_aliases
		 WHERE alias_key = $1 AND (user_id IS NULL OR user_id = $2)
		 ORDER BY user_id NULLS FIRST LIMIT 1`,
		aliasKey, userID,
	).Scan(&variantID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetVariantByID(ctx, variantID)
}

func (r *Repo) StoreAlias(ctx context.Context, userID uuid.UUID, aliasKey string, variantID uuid.UUID) error {
	aliasKey = strings.TrimSpace(strings.ToLower(aliasKey))
	if aliasKey == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO exercise_aliases (user_id, alias_key, variant_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		userID, aliasKey, variantID,
	)
	return err
}

// UserAlias maps alias_key to canonical exercise name (category + variant).
type UserAlias struct {
	AliasKey  string
	Canonical string
}

func (r *Repo) ListUserAliases(ctx context.Context, userID uuid.UUID) ([]UserAlias, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ea.alias_key, ec.name || ' ' || ev.name
		 FROM exercise_aliases ea
		 JOIN exercise_variants ev ON ev.id = ea.variant_id
		 JOIN exercise_categories ec ON ec.id = ev.category_id
		 WHERE ea.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserAlias
	for rows.Next() {
		var a UserAlias
		if err := rows.Scan(&a.AliasKey, &a.Canonical); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanCategories(rows *sql.Rows) ([]*Category, error) {
	var out []*Category
	for rows.Next() {
		var c Category
		var uid sql.NullString
		if err := rows.Scan(&c.ID, &uid, &c.Name, &c.ShowWeight, &c.ShowReps, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.UserID = db.NullStringToUUIDPtr(uid)
		out = append(out, &c)
	}
	return out, rows.Err()
}
