package exercise

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

func titleCase(s string) string {
	var b strings.Builder
	cap := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			cap = true
			b.WriteRune(r)
		} else if cap {
			b.WriteRune(unicode.ToTitle(r))
			cap = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Embedder generates embeddings for text. Nil = no semantic matching.
type Embedder interface {
	Embed(ctx context.Context, userID uuid.UUID, text string) ([]float32, error)
}

// Service provides ResolveOrCreate: exact match → alias lookup → create new.
// We always create when not found; no semantic matching to avoid wrong mappings (e.g. "rack pull" → "bench press").
type Service struct {
	repo     *Repo
	embedder Embedder
}

func NewService(repo *Repo, embedder Embedder) *Service {
	return &Service{repo: repo, embedder: embedder}
}

// ResolveOrCreate resolves (categoryName, variantName) to a Variant, or creates user-level category+variant.
// When variantName is empty, uses the standard variant for that category.
func (s *Service) ResolveOrCreate(ctx context.Context, userID uuid.UUID, categoryName, variantName string) (*Variant, error) {
	categoryName = strings.TrimSpace(strings.ToLower(categoryName))
	variantName = strings.TrimSpace(strings.ToLower(variantName))
	if categoryName == "" {
		return nil, fmt.Errorf("category name required")
	}
	v, err := s.repo.Resolve(ctx, userID, categoryName, variantName)
	if err != nil {
		return nil, err
	}
	if v != nil {
		return v, nil
	}
	aliasKey := categoryName + " " + variantName
	if variantName == "" {
		aliasKey = categoryName + " standard"
	}
	v, err = s.repo.FindVariantByAlias(ctx, userID, aliasKey)
	if err != nil {
		return nil, err
	}
	if v != nil {
		return v, nil
	}

	cat, err := s.repo.GetCategoryByUserAndName(ctx, nil, categoryName)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		cat, err = s.repo.GetCategoryByUserAndName(ctx, &userID, categoryName)
		if err != nil {
			return nil, err
		}
	}

	if cat != nil {
		v, err = s.createVariantForCategory(ctx, userID, cat.ID, categoryName, variantName, false)
	} else {
		v, err = s.createCategoryAndVariant(ctx, userID, categoryName, variantName)
	}
	if err != nil {
		return nil, err
	}
	_ = s.repo.StoreAlias(ctx, userID, aliasKey, v.ID)
	return v, nil
}

func (s *Service) createVariantForCategory(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, categoryName, variantName string, isStandard bool) (*Variant, error) {
	name := variantName
	if name == "" {
		name = "standard"
	}
	v := &Variant{
		CategoryID: categoryID,
		UserID:     &userID,
		Name:       name,
		Standard:   isStandard,
	}
	if err := s.repo.CreateVariant(ctx, v); err != nil {
		return nil, fmt.Errorf("create variant: %w", err)
	}
	if s.embedder != nil {
		emb, err := s.embedder.Embed(ctx, userID, categoryName+" "+name)
		if err == nil && len(emb) > 0 {
			_ = s.repo.UpdateVariantEmbedding(ctx, v.ID, emb)
		}
	}
	return v, nil
}

func (s *Service) createCategoryAndVariant(ctx context.Context, userID uuid.UUID, categoryName, variantName string) (*Variant, error) {
	cat := &Category{
		UserID:     &userID,
		Name:       titleCase(categoryName),
		ShowWeight: true,
		ShowReps:   true,
	}
	if err := s.repo.CreateCategory(ctx, cat); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	return s.createVariantForCategory(ctx, userID, cat.ID, categoryName, variantName, true)
}
