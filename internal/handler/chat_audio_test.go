package handler

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

// TestChat_audioSamples runs sample audio files through the full pipeline (Whisper + Parse + Log).
// Requires OPENAI_API_KEY set and OPENAI_TEST_MODE not "true". Skips if not configured.
// Run with: make test (or go test -v ./internal/handler -run TestChat_audioSamples)
func TestChat_audioSamples(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set, skipping audio integration test")
	}
	if strings.ToLower(os.Getenv("OPENAI_TEST_MODE")) == "true" {
		t.Skip("OPENAI_TEST_MODE=true, skipping audio integration test (use real API)")
	}

	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "audio-" + uuid.New().String(), Email: "audio@test.com", Name: "Audio"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM log_entries WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle)
	parser := ai.NewParser(aiClient)
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	exerciseRepo := exercise.NewRepo(db)
	exerciseSvc := exercise.NewService(exerciseRepo, aiClient)
	prRepo := pr.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)
	querySvc := query.NewService(exerciseRepo, logentryRepo, sessionRepo)
	correctionSvc := correction.NewService(logentryRepo, exerciseRepo)
	prSvc := pr.NewService(prRepo)

	chatSvc := chat.NewService(aiClient, parser, sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo, querySvc, correctionSvc, prSvc, prRepo, nil)

	// Resolve samples path: go test runs from package dir (internal/handler), so go up to module root
	samplesDir := filepath.Join("..", "..", "samples", "audio")
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		t.Skipf("samples/audio not found: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".m4a") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(samplesDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			b64 := base64.StdEncoding.EncodeToString(data)

			resp, err := chatSvc.Process(ctx, u.ID, "", b64, "m4a")
			if err != nil {
				t.Fatalf("Process: %v", err)
			}

			t.Logf("file=%s intent=%s message=%s", name, resp.Intent, resp.Message)
			if len(resp.Entries) > 0 {
				for _, e := range resp.Entries {
					t.Logf("  logged: %s / %s session=%s entry=%s", e.ExerciseName, e.VariantName, e.SessionDate, e.EntryID)
				}
			}
			if len(resp.PRs) > 0 {
				for _, p := range resp.PRs {
					t.Logf("  PR: %s %s %.1f x %v (%s)", p.Exercise, p.Variant, p.Weight, p.Reps, p.PRType)
				}
			}
		})
	}
}
