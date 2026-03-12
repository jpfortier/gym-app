package exercise

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/db"
)

// Resolver resolves (categoryName, variantName) to a Variant using exact match.
// When variantName is empty, returns the standard variant for that category.
// Order: global category first, then user category. For variants: global first, then user.
func (r *Repo) Resolve(ctx context.Context, userID uuid.UUID, categoryName, variantName string) (*Variant, error) {
	categoryName = strings.TrimSpace(strings.ToLower(categoryName))
	variantName = strings.TrimSpace(strings.ToLower(variantName))
	if categoryName == "" {
		return nil, fmt.Errorf("category name required")
	}
	cat, err := r.resolveCategory(ctx, userID, categoryName)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, nil
	}
	if variantName == "" || variantName == "standard" {
		return r.GetStandardVariantByCategory(ctx, cat.ID, userID)
	}
	return r.resolveVariant(ctx, cat.ID, userID, variantName)
}

func (r *Repo) resolveCategory(ctx context.Context, userID uuid.UUID, name string) (*Category, error) {
	cat, err := r.GetCategoryByUserAndName(ctx, nil, name)
	if err != nil {
		return nil, err
	}
	if cat != nil {
		return cat, nil
	}
	return r.GetCategoryByUserAndName(ctx, &userID, name)
}

func (r *Repo) resolveVariant(ctx context.Context, categoryID uuid.UUID, userID uuid.UUID, name string) (*Variant, error) {
	v, err := r.GetVariantByCategoryAndName(ctx, categoryID, nil, name)
	if err != nil {
		return nil, err
	}
	if v != nil {
		return v, nil
	}
	return r.GetVariantByCategoryAndName(ctx, categoryID, &userID, name)
}

// GetStandardVariantByCategory returns the variant with standard=true for the category.
// Order: global (user_id null) first, then user-level.
func (r *Repo) GetStandardVariantByCategory(ctx context.Context, categoryID uuid.UUID, userID uuid.UUID) (*Variant, error) {
	v, err := r.getStandardVariant(ctx, categoryID, nil)
	if err != nil || v != nil {
		return v, err
	}
	return r.getStandardVariant(ctx, categoryID, &userID)
}

func (r *Repo) getStandardVariant(ctx context.Context, categoryID uuid.UUID, userID *uuid.UUID) (*Variant, error) {
	var userVal interface{}
	if userID != nil {
		userVal = *userID
	}
	var v Variant
	var uid, visualCues sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, category_id, user_id, name, standard, visual_cues, created_at FROM exercise_variants
		 WHERE category_id = $1 AND (user_id IS NOT DISTINCT FROM $2) AND standard = true`,
		categoryID, userVal,
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
