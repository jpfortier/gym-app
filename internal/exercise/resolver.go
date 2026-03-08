package exercise

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Resolver resolves (categoryName, variantName) to a Variant using exact match.
// Order: global category first, then user category. For variants: global first, then user.
func (r *Repo) Resolve(ctx context.Context, userID uuid.UUID, categoryName, variantName string) (*Variant, error) {
	categoryName = strings.TrimSpace(strings.ToLower(categoryName))
	variantName = strings.TrimSpace(strings.ToLower(variantName))
	if categoryName == "" || variantName == "" {
		return nil, fmt.Errorf("category and variant names required")
	}
	cat, err := r.resolveCategory(ctx, userID, categoryName)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, nil
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
