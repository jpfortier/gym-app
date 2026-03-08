package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/jpfortier/gym-app/internal/env"
)

// Throttler enforces per-user rate limits for AI API calls.
type Throttler struct {
	mu sync.Mutex
	m  map[string]*userLimit
	// config
	ratePerMinute int
	dailyLimit   int
	dalleLimit   int
}

type userLimit struct {
	limiter   *rate.Limiter
	dayStart  time.Time
	count     int
	dalleCount int
}

func NewThrottlerFromEnv() *Throttler {
	return NewThrottler(
		env.OpenAIRatePerMinute(),
		env.OpenAIDailyLimit(),
		env.OpenAIDalleDailyLimit(),
	)
}

func NewThrottler(ratePerMin, dailyLimit, dalleDailyLimit int) *Throttler {
	if ratePerMin <= 0 {
		ratePerMin = 10
	}
	if dailyLimit <= 0 {
		dailyLimit = 100
	}
	if dalleDailyLimit <= 0 {
		dalleDailyLimit = 5
	}
	return &Throttler{
		m:             make(map[string]*userLimit),
		ratePerMinute: ratePerMin,
		dailyLimit:   dailyLimit,
		dalleLimit:   dalleDailyLimit,
	}
}

func (t *Throttler) getOrCreate(userID uuid.UUID) *userLimit {
	key := userID.String()
	t.mu.Lock()
	defer t.mu.Unlock()
	ul, ok := t.m[key]
	if !ok {
		ul = &userLimit{
			limiter:  rate.NewLimiter(rate.Limit(t.ratePerMinute), t.ratePerMinute),
			dayStart: time.Now().Truncate(24 * time.Hour),
		}
		t.m[key] = ul
	}
	now := time.Now()
	if now.Sub(ul.dayStart) >= 24*time.Hour {
		ul.dayStart = now.Truncate(24 * time.Hour)
		ul.count = 0
		ul.dalleCount = 0
	}
	return ul
}

// Allow checks if the user can make an AI request (Whisper/GPT). Returns error if throttled.
func (t *Throttler) Allow(ctx context.Context, userID uuid.UUID) error {
	ul := t.getOrCreate(userID)
	t.mu.Lock()
	if ul.count >= t.dailyLimit {
		t.mu.Unlock()
		return fmt.Errorf("daily AI limit reached (%d)", t.dailyLimit)
	}
	t.mu.Unlock()
	if err := ul.limiter.Wait(ctx); err != nil {
		return err
	}
	t.mu.Lock()
	ul.count++
	t.mu.Unlock()
	return nil
}

// AllowDalle checks if the user can generate a DALL-E image. Returns error if throttled.
func (t *Throttler) AllowDalle(ctx context.Context, userID uuid.UUID) error {
	ul := t.getOrCreate(userID)
	t.mu.Lock()
	if ul.dalleCount >= t.dalleLimit {
		t.mu.Unlock()
		return fmt.Errorf("daily DALL-E limit reached (%d)", t.dalleLimit)
	}
	ul.dalleCount++
	t.mu.Unlock()
	return nil
}
