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

// Service provides ResolveOrCreate: exact match → embedding match → create new.
type Service struct {
	repo     *Repo
	embedder Embedder
}

func NewService(repo *Repo, embedder Embedder) *Service {
	return &Service{repo: repo, embedder: embedder}
}

// ResolveOrCreate resolves (categoryName, variantName) to a Variant, or creates user-level category+variant.
func (s *Service) ResolveOrCreate(ctx context.Context, userID uuid.UUID, categoryName, variantName string) (*Variant, error) {
	categoryName = strings.TrimSpace(strings.ToLower(categoryName))
	variantName = strings.TrimSpace(strings.ToLower(variantName))
	if categoryName == "" || variantName == "" {
		return nil, fmt.Errorf("category and variant names required")
	}
	v, err := s.repo.Resolve(ctx, userID, categoryName, variantName)
	if err != nil {
		return nil, err
	}
	if v != nil {
		return v, nil
	}
	if s.embedder != nil {
		v, err = s.resolveByEmbedding(ctx, userID, categoryName, variantName)
		if err != nil {
			return nil, err
		}
		if v != nil {
			return v, nil
		}
	}
	return s.createCategoryAndVariant(ctx, userID, categoryName, variantName)
}

func (s *Service) resolveByEmbedding(ctx context.Context, userID uuid.UUID, categoryName, variantName string) (*Variant, error) {
	emb, err := s.embedder.Embed(ctx, userID, categoryName+" "+variantName)
	if err != nil || len(emb) == 0 {
		return nil, nil
	}
	cat, err := s.repo.FindCategoryByEmbedding(ctx, userID, emb, 0.2)
	if err != nil || cat == nil {
		return nil, err
	}
	v, err := s.repo.FindVariantByEmbedding(ctx, cat.ID, userID, emb, 0.2)
	if err != nil || v == nil {
		return nil, err
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
	v := &Variant{
		CategoryID: cat.ID,
		UserID:     &userID,
		Name:       variantName,
	}
	if err := s.repo.CreateVariant(ctx, v); err != nil {
		return nil, fmt.Errorf("create variant: %w", err)
	}
	if s.embedder != nil {
		emb, err := s.embedder.Embed(ctx, userID, categoryName+" "+variantName)
		if err == nil && len(emb) > 0 {
			if err := s.repo.UpdateCategoryEmbedding(ctx, cat.ID, emb); err != nil {
				return nil, fmt.Errorf("update category embedding: %w", err)
			}
			if err := s.repo.UpdateVariantEmbedding(ctx, v.ID, emb); err != nil {
				return nil, fmt.Errorf("update variant embedding: %w", err)
			}
		}
	}
	return v, nil
}
