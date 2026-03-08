package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// GetOrCreateForDate returns the session for the user on the given date (YYYY-MM-DD), creating it if it doesn't exist.
func (s *Service) GetOrCreateForDate(ctx context.Context, userID uuid.UUID, date string) (*Session, error) {
	dateNorm, err := normalizeDate(date)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByUserAndDate(ctx, userID, dateNorm)
	if err != nil {
		return nil, fmt.Errorf("get by user and date: %w", err)
	}
	if existing != nil {
		return existing, nil
	}
	parsed, err := time.Parse("2006-01-02", dateNorm)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}
	sess := &Session{UserID: userID, Date: parsed}
	if err := s.repo.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

func normalizeDate(s string) (string, error) {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10], nil
	}
	return "", fmt.Errorf("date must be YYYY-MM-DD")
}
